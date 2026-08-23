package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, cfg config.Config) (*Store, error) {
	if cfg.PostgresPassword == "" {
		return nil, fmt.Errorf("postgres password not configured")
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?pool_max_conns=10",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}
