package config

import (
	"os"

	"github.com/rawbytedev/zerokv/encoders"
)

type Config struct {
	PostgresDSN          string
	CacheDir             string
	SolanaRPCURL         string
	AdminPrivateKey      string
	MobileMoneyAPIURL    string
	MobileMoneyAPIKey    string
	MobileMoneyAPISecret string
}

func NewConfig() *Config {
	postgresDSN := os.Getenv("DATABASE_URL")
	if postgresDSN == "" {
		postgresDSN = "host=localhost user=postgres password=secret dbname=diaspora port=5432 sslmode=disable"
	}

	adminPrivateKey := os.Getenv("ADMIN_PRIVATE_KEY")
	if adminPrivateKey == "" {
		adminPrivateKey = "your_admin_private_key"
	}

	mobileMoneyAPIKey := os.Getenv("MOBILE_MONEY_API_KEY")
	if mobileMoneyAPIKey == "" {
		mobileMoneyAPIKey = "your_api_key"
	}

	mobileMoneyAPISecret := os.Getenv("MOBILE_MONEY_API_SECRET")
	if mobileMoneyAPISecret == "" {
		mobileMoneyAPISecret = "your_api_secret"
	}

	solanaRPCURL := os.Getenv("SOLANA_RPC_URL")
	if solanaRPCURL == "" {
		solanaRPCURL = "https://api.devnet.solana.com"
	}

	return &Config{
		PostgresDSN:          postgresDSN,
		CacheDir:             "./badger_data",
		SolanaRPCURL:         solanaRPCURL,
		AdminPrivateKey:      adminPrivateKey,
		MobileMoneyAPIURL:    "https://api.mobilemoney.com",
		MobileMoneyAPIKey:    mobileMoneyAPIKey,
		MobileMoneyAPISecret: mobileMoneyAPISecret,
	}
}

func (c *Config) LoadFromEnv() {
	if adminPrivateKey := os.Getenv("ADMIN_PRIVATE_KEY"); adminPrivateKey != "" {
		c.AdminPrivateKey = adminPrivateKey
	}
	if mobileMoneyAPIKey := os.Getenv("MOBILE_MONEY_API_KEY"); mobileMoneyAPIKey != "" {
		c.MobileMoneyAPIKey = mobileMoneyAPIKey
	}
	if mobileMoneyAPISecret := os.Getenv("MOBILE_MONEY_API_SECRET"); mobileMoneyAPISecret != "" {
		c.MobileMoneyAPISecret = mobileMoneyAPISecret
	}
}

func (c *Config) Validate() error {
	return nil
}

func (c *Config) SaveToFile(filename string) error {
	enc := encoders.NewJsonEncoder()
	data, err := enc.Encode(c)
	if err != nil {
		return err
	}
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

func LoadConfigFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	enc := encoders.NewJsonEncoder()
	var config Config
	err = enc.Decode(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func LoadConfig() *Config {
	return NewConfig()
}
