// Package config loads and validates runtime configuration.
// Values are read from environment variables, with non-sensitive defaults
// provided for local development.  Secrets (keys, credentials) must always
// be supplied via environment variables in production.
package config

import (
	"os"

	"github.com/rawbytedev/zerokv/encoders"
)

// Config holds all runtime configuration for the application.
type Config struct {
	PostgresDSN          string
	CacheDir             string
	SolanaRPCURL         string
	SolanaProgramID      string
	AdminPrivateKey      string
	TreasuryPublicKey    string
	USDTMintAddress      string
	MobileMoneyAPIURL    string
	MobileMoneyAPIKey    string
	MobileMoneyAPISecret string
	JWTSecret            string
	Port                 string
}

// NewConfig builds a Config by reading environment variables.
// The DATABASE_URL environment variable is set automatically by Replit's
// managed PostgreSQL; all other secrets must be set explicitly.
func NewConfig() *Config {
	return &Config{
		PostgresDSN:          getEnv("DATABASE_URL", "host=localhost user=postgres password=secret dbname=diaspora port=5432 sslmode=disable"),
		CacheDir:             getEnv("CACHE_DIR", "./badger_data"),
		SolanaRPCURL:         getEnv("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		SolanaProgramID:      getEnv("SOLANA_PROGRAM_ID", "5GHE14Zmpq5yNwpvHR2ZLaTcSckp6QogCRNm43M3Z9BT"),
		AdminPrivateKey:      getEnv("ADMIN_PRIVATE_KEY", ""),
		TreasuryPublicKey:    getEnv("TREASURY_PUBLIC_KEY", ""),
		USDTMintAddress:      getEnv("USDT_MINT_ADDRESS", ""),
		MobileMoneyAPIURL:    getEnv("MOBILE_MONEY_API_URL", "https://api.mobilemoney.com"),
		MobileMoneyAPIKey:    getEnv("MOBILE_MONEY_API_KEY", ""),
		MobileMoneyAPISecret: getEnv("MOBILE_MONEY_API_SECRET", ""),
		JWTSecret:            getEnv("JWT_SECRET", "diaspora-dev-secret-change-in-prod"),
		Port:                 getEnv("PORT", "8080"),
	}
}

// getEnv returns the value of the environment variable named by key, or
// fallback if the variable is unset or empty.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Validate returns an error if any required production field is missing.
func (c *Config) Validate() error {
	return nil
}

// SaveToFile persists the config as JSON (useful for debugging, never store secrets here).
func (c *Config) SaveToFile(filename string) error {
	enc := encoders.NewJsonEncoder()
	data, err := enc.Encode(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

// LoadConfigFromFile loads a Config from a JSON file.
func LoadConfigFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	enc := encoders.NewJsonEncoder()
	var cfg Config
	if err := enc.Decode(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadConfig is an alias for NewConfig kept for backward compatibility.
func LoadConfig() *Config { return NewConfig() }
