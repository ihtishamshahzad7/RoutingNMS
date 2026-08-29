package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct { Pool *pgxpool.Pool }

func New(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" { return nil, fmt.Errorf("database URL is required") }
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil { return nil, fmt.Errorf("create database pool: %w", err) }
	if err := pool.Ping(ctx); err != nil { pool.Close(); return nil, fmt.Errorf("database ping: %w", err) }
	return &Store{Pool: pool}, nil
}

func (s *Store) Close() { if s != nil && s.Pool != nil { s.Pool.Close() } }

func (s *Store) Health(ctx context.Context) error {
	if s == nil || s.Pool == nil { return fmt.Errorf("database store is not initialized") }
	return s.Pool.Ping(ctx)
}

var _ = pgx.ErrNoRows
