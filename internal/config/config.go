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
	return &Config{
		PostgresDSN:          "host=localhost user=postgres password=secret dbname=diaspora port=5432 sslmode=disable",
		CacheDir:             "./badger_data",
		SolanaRPCURL:         "https://api.devnet.solana.com",
		AdminPrivateKey:      "your_admin_private_key", // In a real implementation, this should be securely stored and not hardcoded
		MobileMoneyAPIURL:    "https://api.mobilemoney.com",
		MobileMoneyAPIKey:    "your_api_key",    // In
		MobileMoneyAPISecret: "your_api_secret", // In a real implementation, this should be securely stored and not hardcoded
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
	// In a real implementation, you would add validation logic here to ensure all required fields are set and valid
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
	adminPrivateKey := os.Getenv("ADMIN_PRIVATE_KEY")
	if adminPrivateKey == "" {
		adminPrivateKey = "your_admin_private_key" // In a real implementation, this should be securely stored and not hardcoded
	}
	MobileMoneyAPIKey := os.Getenv("MOBILE_MONEY_API_KEY")
	if MobileMoneyAPIKey == "" {
		MobileMoneyAPIKey = "your_api_key" // In a real implementation, this should be securely stored and not hardcoded
	}
	MobileMoneyAPISecret := os.Getenv("MOBILE_MONEY_API_SECRET")
	if MobileMoneyAPISecret == "" {
		MobileMoneyAPISecret = "your_api_secret" // In a real implementation, this should be securely stored and not hardcoded
	}

	return &Config{
		PostgresDSN:          "host=localhost user=postgres password=secret dbname=diaspora port=5432 sslmode=disable",
		CacheDir:             "./badger_data",
		SolanaRPCURL:         "https://api.devnet.solana.com",
		AdminPrivateKey:      adminPrivateKey,
		MobileMoneyAPIURL:    "https://api.mobilemoney.com",
		MobileMoneyAPIKey:    MobileMoneyAPIKey,
		MobileMoneyAPISecret: MobileMoneyAPISecret,
	}
}
