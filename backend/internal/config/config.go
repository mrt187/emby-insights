package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains only server-side configuration. Secrets must never reach the browser.
type Config struct {
	ListenAddress           string
	DatabaseURL             string
	RedisURL                string
	EmbyBaseURL             string
	EmbyDeviceID            string
	EmbyAdminAPIKey         string
	RadarrBaseURL           string
	RadarrAPIKey            string
	SonarrBaseURL           string
	SonarrAPIKey            string
	TmdbAPIKey              string
	ComingSoonRegion        string
	ComingSoonDaysAhead     int
	EmbyNewForYouLibraryIDs []string
	EmbyWatchedLibraryIDs   []string
	SeerrBaseURL            string
	SeerrAPIKey             string
	CookieSecure            bool
	ShutdownTimeout         time.Duration
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

	embyDeviceID, err := required("EMBY_DEVICE_ID")
	if err != nil {
		return Config{}, err
	}

	embyAdminAPIKey, err := required("EMBY_ADMIN_API_KEY")
	if err != nil {
		return Config{}, err
	}

	return Config{
		ListenAddress:           valueOr("LISTEN_ADDRESS", ":8080"),
		DatabaseURL:             databaseURL,
		RedisURL:                redisURL,
		EmbyBaseURL:             embyBaseURL,
		EmbyDeviceID:            embyDeviceID,
		EmbyAdminAPIKey:         embyAdminAPIKey,
		RadarrBaseURL:           valueOr("RADARR_URL", ""),
		RadarrAPIKey:            valueOr("RADARR_API_KEY", ""),
		SonarrBaseURL:           valueOr("SONARR_URL", ""),
		SonarrAPIKey:            valueOr("SONARR_API_KEY", ""),
		TmdbAPIKey:              valueOr("TMDB_API_KEY", ""),
		ComingSoonRegion:        valueOr("COMINGSOON_REGION", "DE"),
		ComingSoonDaysAhead:     intValue("COMINGSOON_DAYS_AHEAD", 28),
		EmbyNewForYouLibraryIDs: splitList(valueOr("EMBY_NEW_FOR_YOU_LIBRARY_IDS", "")),
		EmbyWatchedLibraryIDs:   splitList(valueOr("EMBY_WATCHED_LIBRARY_IDS", "")),
		SeerrBaseURL:            valueOr("SEERR_URL", ""),
		SeerrAPIKey:             valueOr("SEERR_API_KEY", ""),
		CookieSecure:            valueOr("COOKIE_SECURE", "true") != "false",
		ShutdownTimeout:         10 * time.Second,
	}, nil
}

func intValue(name string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(name)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

// splitList parses an optional comma-separated library selection. An empty
// result disables that scoped card instead of failing startup.
func splitList(value string) []string {
	if value == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
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
