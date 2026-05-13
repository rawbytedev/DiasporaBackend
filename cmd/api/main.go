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
	// Load configuration
	cfg := config.NewConfig()
	// Connexion PostgreSQL
	// 
	dbPost, err := db.NewPostgresDB(cfg.PostgresDSN)
	if err != nil {
		log.Fatal(err)
	}
	// Cache BadgerDB
	cacheDB, err := cache.NewCache(cfg.CacheDir, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cacheDB.Close()
	// Repos
	userRepo := repository.NewUserRepo(cacheDB, dbPost)
	transferRepo := repository.NewTransferRepo(cacheDB, dbPost)
	// Client Solana (devnet)
	solClient, err := solana.NewClient(cfg.SolanaRPCURL, dbPost, cfg.AdminPrivateKey) // In a real implementation, the server would have its own Solana wallet private key for signing transactions
	if err != nil {
		log.Fatal(err)
	}
	mobilemoneyClient := mobilemoney.NewClient("https://api.mobilemoney.com", cfg.MobileMoneyAPIKey, cfg.MobileMoneyAPISecret) // Placeholder URL and API key
	// Routes
	http.HandleFunc("/api/register", handlers.Register(userRepo))
	http.HandleFunc("/api/verify-otp", handlers.VerifyOTP(userRepo))
	http.HandleFunc("/api/transfer", middleware.AuthMiddleware(handlers.SendTransfer(userRepo, transferRepo, solClient)))
	http.HandleFunc("/api/claim", middleware.AuthMiddleware(handlers.ClaimTransfer(transferRepo, userRepo, solClient, dbPost)))
	http.HandleFunc("/api/withdraw", middleware.AuthMiddleware(handlers.Withdraw(userRepo, mobilemoneyClient)))
	// Logging middleware global
	loggedMux := middleware.LoggingMiddleware(http.DefaultServeMux)
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", loggedMux))
}
