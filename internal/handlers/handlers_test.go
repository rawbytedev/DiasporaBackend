package handlers_test

import (
	"Diaspora/internal/cache"
	"Diaspora/internal/db"
	"Diaspora/internal/handlers"
	"Diaspora/internal/repository"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func SetupTestDB(path string) (*repository.UserRepo, *db.PostgresDB, error) {
	dsn := "host=localhost port=5432 user=AdminDias password=Admin dbname=diaspora_test sslmode=disable"
	store, err := cache.NewCache(path, nil)
	if err != nil {
		return nil, nil, err
	}
	db, err := db.NewPostgresDB(dsn)
	if err != nil {
		return nil, nil, err
	}
	return repository.NewUserRepo(store, db), db, nil
}

// tableExists checks if a table exists in the 'public' schema
func tableExists(ctx context.Context, conn *pgx.Conn, table string) (bool, error) {
	if table == "" {
		return false, fmt.Errorf("table name cannot be empty")
	}

	var exists bool
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_name = $1
		)
	`
	err := conn.QueryRow(ctx, query, table).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func CreateTable(t *testing.T, db *db.PostgresDB) {
	createTableSQL := `CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    password VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20) UNIQUE NOT NULL,
    solana_pubkey VARCHAR(44) NOT NULL,
    encrypted_priv_key TEXT NOT NULL,
    name VARCHAR(100),
    state_version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`
	dbTx, err := db.BeginTx(context.Background())
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v\n", err)
	}
	tableExists, err := tableExists(t.Context(), dbTx.Conn(), "users")
	if err != nil {
		t.Fatalf("Failed to check table existence: %v\n", err)
	}
	dbTx.Rollback(t.Context())
	// Only create the table if it doesn't exist
	if tableExists {
		t.Logf("Table 'users' already exists, skipping creation.\n")
	}
	if !tableExists {
		t.Logf("Table 'users' does not exist, creating table.\n")
		dbTx, err = db.BeginTx(context.Background())
		if err != nil {
			t.Fatalf("Failed to begin transaction: %v\n", err)
		}
		_, err = dbTx.Exec(t.Context(), createTableSQL)
		if err != nil {
			log.Fatalf("Failed to create table: %v\n", err)
		}
		dbTx.Commit(t.Context())
		t.Logf("Table 'users' created successfully.\n")
	}

}

func TestCreateUser(t *testing.T) {
	userRepo, db, err := SetupTestDB(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	CreateTable(t, db)
	userRepo.Close()
}

func EndPoints(t *testing.T) *repository.UserRepo {
	userRepo, db, err := SetupTestDB(t.TempDir())
	_ = db
	if err != nil {
		t.Fatal()
	}
	return userRepo
}
func TestRegister(t *testing.T) {
	userRepo := EndPoints(t)
	handlers := handlers.Register(userRepo)
	reqBody := "phone=1234567890&name=TestUser&password=TestPass"
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handlers(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	userRepo.Close()
}

func TestLogin(t *testing.T) {
	userRepo := EndPoints(t)
	handlers := handlers.Login(userRepo)
	reqBody := "phone=1234567890&password=TestPass"
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	handlers(w, req)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	userRepo.Close()
}
