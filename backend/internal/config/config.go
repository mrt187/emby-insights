package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config contains only the values needed before the database is reachable.
// Everything the setup wizard can configure at runtime (Emby device id,
// library selection, Seerr/Radarr/Sonarr/TMDB) lives in Postgres instead —
// see internal/appconfig. Secrets here must never reach the browser.
type Config struct {
	ListenAddress    string
	DatabaseURL      string
	RedisURL         string
	EmbyBaseURL      string
	EmbyAdminAPIKey  string
	AppEncryptionKey string
	CookieSecure     bool
	ShutdownTimeout  time.Duration
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

	embyBaseURL, err := required("EMBY_BASE_URL")
	if err != nil {
		return Config{}, err
	}

	embyAdminAPIKey, err := required("EMBY_ADMIN_API_KEY")
	if err != nil {
		return Config{}, err
	}

	appEncryptionKey, err := required("APP_ENCRYPTION_KEY")
	if err != nil {
		return Config{}, err
	}

	return Config{
		ListenAddress:    valueOr("LISTEN_ADDRESS", ":8080"),
		DatabaseURL:      databaseURL,
		RedisURL:         redisURL,
		EmbyBaseURL:      embyBaseURL,
		EmbyAdminAPIKey:  embyAdminAPIKey,
		AppEncryptionKey: appEncryptionKey,
		CookieSecure:     valueOr("COOKIE_SECURE", "true") != "false",
		ShutdownTimeout:  10 * time.Second,
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
