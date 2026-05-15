package config

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNewConfigDefaults(t *testing.T) {
	oldDatabaseURL := os.Getenv("DATABASE_URL")
	oldJWTSecret := os.Getenv("JWT_SECRET")
	defer func() {
		os.Setenv("DATABASE_URL", oldDatabaseURL)
		os.Setenv("JWT_SECRET", oldJWTSecret)
	}()

	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET")

	cfg := NewConfig()

	if cfg.PostgresDSN == "" {
		t.Fatal("expected default PostgresDSN, got empty string")
	}
	if cfg.JWTSecret != "diaspora-dev-secret-change-in-prod" {
		t.Fatalf("expected default JWT secret, got %q", cfg.JWTSecret)
	}
}

func TestSaveLoadConfigFile(t *testing.T) {
	cfg := &Config{
		PostgresDSN:          "host=test",
		CacheDir:             "./cache",
		SolanaRPCURL:         "https://api.testnet.solana.com",
		SolanaProgramID:      "ProgramID",
		AdminPrivateKey:      "private-key",
		TreasuryPublicKey:    "treasury-key",
		USDTMintAddress:      "usdt-mint",
		MobileMoneyAPIURL:    "https://api.example.com",
		MobileMoneyAPIKey:    "api-key",
		MobileMoneyAPISecret: "api-secret",
		JWTSecret:            "jwt-secret",
		Port:                 "9000",
	}

	path := t.TempDir() + "/config.json"
	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loaded, err := LoadConfigFromFile(path)
	if err != nil {
		t.Fatalf("LoadConfigFromFile failed: %v", err)
	}

	if loaded.PostgresDSN != cfg.PostgresDSN {
		t.Fatalf("expected PostgresDSN %q, got %q", cfg.PostgresDSN, loaded.PostgresDSN)
	}
	if loaded.Port != cfg.Port {
		t.Fatalf("expected Port %q, got %q", cfg.Port, loaded.Port)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	var toCheck map[string]interface{}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}
	if err := json.Unmarshal(data, &toCheck); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
}
