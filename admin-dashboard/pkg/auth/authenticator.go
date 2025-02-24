package auth

import (
	"admin-dashboard/config"
)

type Authenticator struct {
	config *config.Config
}

func NewAuthenticator(cfg *config.Config) *Authenticator {
	return &Authenticator{
		config: cfg,
	}
}

func (a *Authenticator) ValidateToken(token string) (*User, error) {
	// Implementation
	return nil, nil
}
