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

	// Web Push (VAPID). PushPublicKey is not a secret — it goes straight to
	// the browser via GET /api/push/public-key — but the private key must
	// never leave the server.
	PushPublicKey    string
	PushPrivateKey   string
	PushSubject      string
	PushPollInterval time.Duration

	// TrustedProxies lists the IPs/CIDRs whose X-Forwarded-For header may be
	// believed when identifying the client for login throttling. Empty means
	// trust nobody and always use the peer address — correct for a directly
	// exposed container, and the safe default when the value is unset.
	TrustedProxies []string
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

	pushPublicKey, err := required("VAPID_PUBLIC_KEY")
	if err != nil {
		return Config{}, err
	}

	pushPrivateKey, err := required("VAPID_PRIVATE_KEY")
	if err != nil {
		return Config{}, err
	}

	pushSubject, err := required("VAPID_SUBJECT")
	if err != nil {
		return Config{}, err
	}

	pushPollInterval, err := durationOr("PUSH_POLL_INTERVAL", 20*time.Minute)
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
		TrustedProxies:   splitList(os.Getenv("TRUSTED_PROXIES")),
		PushPublicKey:    pushPublicKey,
		PushPrivateKey:   pushPrivateKey,
		PushSubject:      pushSubject,
		PushPollInterval: pushPollInterval,
	}, nil
}

func splitList(value string) []string {
	var entries []string
	for _, entry := range strings.Split(value, ",") {
		if entry = strings.TrimSpace(entry); entry != "" {
			entries = append(entries, entry)
		}
	}
	return entries
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

func durationOr(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration (e.g. 20m): %w", name, err)
	}
	return parsed, nil
}
