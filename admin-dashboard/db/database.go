package db

import (
	"admin-dashboard/models"
	"context"
	"database/sql"
)

type Database struct {
	db *sql.DB
}

func New(connStr string) (*Database, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// Add database methods here
func (d *Database) GetMessages(ctx context.Context, clientID string) ([]models.Message, error) {
	// Implementation
	return nil, nil
}
