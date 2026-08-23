package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/config"
	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(ctx context.Context, cfg config.Config) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort)
	opts := &redis.Options{
		Addr:         addr,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	if cfg.RedisPassword != "" {
		opts.Password = cfg.RedisPassword
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping %s: %w", addr, err)
	}
	return &Client{rdb: rdb}, nil
}

func (c *Client) Raw() *redis.Client {
	return c.rdb
}

func (c *Client) Close() error {
	if c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}
