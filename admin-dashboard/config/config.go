package config

import (
	"os"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Auth     AuthConfig
	Meta     MetaConfig
	Server   ServerConfig
	Messages MessagesConfig
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

type MessagesConfig struct {
	DefaultPageSize int
	MaxPageSize     int
	CacheTimeout    time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Messages: MessagesConfig{
			DefaultPageSize: 30,
			MaxPageSize:     100,
			CacheTimeout:    5 * time.Minute,
		},
	}

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
