package config

import (
	"os"
)

type Config struct {
	Database struct {
		URL      string
		MaxConns int
	}
	Server struct {
		Port    string
		BaseURL string
	}
	Auth struct {
		JWTSecret string
		TokenTTL  int
	}
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Load from environment variables
	cfg.Database.URL = os.Getenv("DATABASE_URL")
	cfg.Server.Port = os.Getenv("PORT")
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	cfg.Auth.JWTSecret = os.Getenv("JWT_SECRET")

	return cfg, nil
}
