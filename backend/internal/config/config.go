package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config contains only server-side configuration. Secrets must never reach the browser.
type Config struct {
	ListenAddress   string
	DatabaseURL     string
	RedisURL        string
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	databaseURL, err := required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}

	redisURL, err := required("REDIS_URL")
	if err != nil {
		return Config{}, err
	}

	return Config{
		ListenAddress:   valueOr("LISTEN_ADDRESS", ":8080"),
		DatabaseURL:     databaseURL,
		RedisURL:        redisURL,
		ShutdownTimeout: 10 * time.Second,
	}, nil
}

func required(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s must be configured", name)
	}
	return value, nil
}

func valueOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
