package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/redis/go-redis/v9"
)

const defaultRedisPingTimeout = 3 * time.Second

type Data struct {
	Redis  *redis.Client
	logger *slog.Logger
}

func New(cfg config.Config, logger *slog.Logger) (*Data, error) {
	opts, err := redisOptionsFromDSN(cfg.RedisDSN)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), defaultRedisPingTimeout)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	if logger != nil {
		logger.Info("redis client initialized", slog.String("addr", opts.Addr), slog.Int("db", opts.DB))
	}
	return &Data{Redis: client, logger: logger}, nil
}

func (d *Data) Close() error {
	if d == nil || d.Redis == nil {
		return nil
	}
	if err := d.Redis.Close(); err != nil {
		return fmt.Errorf("close redis: %w", err)
	}
	if d.logger != nil {
		d.logger.Info("redis client closed")
	}
	return nil
}

func redisOptionsFromDSN(dsn string) (*redis.Options, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("redis dsn is required")
	}
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse redis dsn: %w", err)
	}
	return opts, nil
}
