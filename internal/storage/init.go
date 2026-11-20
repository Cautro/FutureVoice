package storage

import (
	"context"
	"database/sql"
	"fmt"
)

type Storage struct {
	DB *sql.DB
}

func New(ctx context.Context, databaseURL string) (*Storage, error) {
	db, err := openPostgres(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	store := &Storage{DB: db}
	if err := store.initSchema(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize database schema: %w", err)
	}
	return store, nil
}

func (s *Storage) Close() error {
	return s.DB.Close()
}
