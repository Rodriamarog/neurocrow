package config

import (
	"os"
)

type Config struct {
	Database DatabaseConfig
	Auth     AuthConfig
	Meta     MetaConfig
	Server   ServerConfig
}

type ServerConfig struct {
	Port string
}

type MetaConfig struct {
	APIKey string
}

type DatabaseConfig struct {
	URL string
}

type AuthConfig struct {
	Secret string
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Load from environment variables
	cfg.Database.URL = os.Getenv("DATABASE_URL")
	cfg.Server.Port = os.Getenv("PORT")
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	cfg.Auth.Secret = os.Getenv("JWT_SECRET")
	cfg.Meta.APIKey = os.Getenv("META_API_KEY")

	return cfg, nil
}
