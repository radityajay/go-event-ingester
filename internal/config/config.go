package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	Server   ServerConfig
	Redis    RedisConfig
	Postgres PostgresConfig
	Worker   WorkerConfig
}

type ServerConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	Stream   string
	Group    string
}

type PostgresConfig struct {
	DSN           string
	MaxOpenConns  int
	MaxIdleConns  int
	BatchSize     int
	FlushInterval time.Duration
}

type WorkerConfig struct {
	PoolSize      int
	BatchSize     int
	FlushInterval time.Duration
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         envOrDefault("SERVER_PORT", "8080"),
			ReadTimeout:  envDurationOrDefault("SERVER_READ_TIMEOUT", 5*time.Second),
			WriteTimeout: envDurationOrDefault("SERVER_WRITE_TIMEOUT", 10*time.Second),
		},
		Redis: RedisConfig{
			Addr:     envOrDefault("REDIS_ADDR", "localhost:6379"),
			Password: envOrDefault("REDIS_PASSWORD", ""),
			DB:       envIntOrDefault("REDIS_DB", 0),
			Stream:   envOrDefault("REDIS_STREAM", "events"),
			Group:    envOrDefault("REDIS_GROUP", "event-processors"),
		},
		Postgres: PostgresConfig{
			DSN:          envOrDefault("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/eventdb?sslmode=disable"),
			MaxOpenConns: envIntOrDefault("POSTGRES_MAX_OPEN_CONNS", 25),
			MaxIdleConns: envIntOrDefault("POSTGRES_MAX_IDLE_CONNS", 5),
		},
		Worker: WorkerConfig{
			PoolSize:      envIntOrDefault("WORKER_POOL_SIZE", 4),
			BatchSize:     envIntOrDefault("WORKER_BATCH_SIZE", 500),
			FlushInterval: envDurationOrDefault("WORKER_FLUSH_INTERVAL", 2*time.Second),
		},
	}

	if cfg.Redis.Addr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}
	if cfg.Postgres.DSN == "" {
		return nil, fmt.Errorf("POSTGRES_DSN is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return i
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
