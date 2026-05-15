// DiasporaConnect API – entry point.
//
// The server registers all HTTP routes, wires up the PostgreSQL and BadgerDB
// connections, and starts listening.  Configuration is loaded entirely from
// environment variables; see internal/config for the full list.
package main

import (
	"Diaspora/internal/cache"
	"Diaspora/internal/config"
	"Diaspora/internal/db"
	"Diaspora/internal/handlers"
	"Diaspora/internal/middleware"
	"Diaspora/internal/mobilemoney"
	"Diaspora/internal/repository"
	"Diaspora/internal/solana"
	"log"
	"net/http"
)

func main() {
	cfg := config.NewConfig()

	// ── PostgreSQL ────────────────────────────────────────────────────────────
	dbPost, err := db.NewPostgresDB(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres connect: %v", err)
	}

	// ── BadgerDB cache ────────────────────────────────────────────────────────
	cacheDB, err := cache.NewCache(cfg.CacheDir, nil)
	if err != nil {
		log.Fatalf("badger open: %v", err)
	}
	defer cacheDB.Close()

	// ── Solana client ─────────────────────────────────────────────────────────
	solClient, err := solana.NewClient(
		cfg.SolanaRPCURL,
		dbPost,
		cfg.AdminPrivateKey,
		cfg.SolanaProgramID,
		cfg.USDTMintAddress,
		cfg.TreasuryPublicKey,
	)
	if err != nil {
		log.Fatalf("solana client: %v", err)
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepo(cacheDB, dbPost, solClient)
	transferRepo := repository.NewTransferRepo(cacheDB, dbPost)

	// ── Mobile money client ───────────────────────────────────────────────────
	mmClient := mobilemoney.NewClient(cfg.MobileMoneyAPIURL, cfg.MobileMoneyAPIKey, cfg.MobileMoneyAPISecret)

	// ── Routes ───────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Public endpoints
	mux.HandleFunc("/api/register", handlers.Register(userRepo))
	mux.HandleFunc("/api/login", handlers.Login(userRepo))
	mux.HandleFunc("/api/verify-otp", handlers.VerifyOTP(userRepo))

	// Health check
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		middleware.JSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Authenticated endpoints
	mux.HandleFunc("/api/account", middleware.AuthMiddleware(handlers.GetAccount(userRepo)))
	mux.HandleFunc("/api/balance", middleware.AuthMiddleware(handlers.GetBalance(userRepo, solClient)))
	mux.HandleFunc("/api/transfers", middleware.AuthMiddleware(handlers.GetTransfers(transferRepo)))
	mux.HandleFunc("/api/transfers/detail", middleware.AuthMiddleware(handlers.GetTransfer(transferRepo)))
	mux.HandleFunc("/api/transfer", middleware.AuthMiddleware(handlers.SendTransfer(userRepo, transferRepo, solClient)))
	mux.HandleFunc("/api/claim", middleware.AuthMiddleware(handlers.ClaimTransfer(transferRepo, userRepo, solClient, dbPost)))
	mux.HandleFunc("/api/refund", middleware.AuthMiddleware(handlers.RefundTransfer(transferRepo, userRepo, solClient, dbPost)))
	mux.HandleFunc("/api/withdraw", middleware.AuthMiddleware(handlers.Withdraw(userRepo, mmClient)))

	// ── Global middleware ─────────────────────────────────────────────────────
	loggedMux := middleware.LoggingMiddleware(mux)

	addr := ":" + cfg.Port
	log.Printf("DiasporaConnect API listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, loggedMux))
}
