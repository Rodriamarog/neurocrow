package db

import (
	"admin-dashboard/models"
	"context"
	"database/sql"
)

type Database struct {
	DB *sql.DB
}

func New(url string) (*Database, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	return &Database{DB: db}, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

// Add database methods here
func (d *Database) GetMessages(ctx context.Context, clientID string) ([]models.Message, error) {
	// Implementation
	return nil, nil
}
