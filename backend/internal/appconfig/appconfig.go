// Package appconfig replaces manual .env configuration for the optional
// Seerr/Radarr/Sonarr/TMDB integrations, Emby library selection, and the
// Emby Insights admin identity. All of it now lives in Postgres (singleton
// rows in admin_owner and app_config) and is editable at runtime through the
// Verwaltung admin UI instead of requiring a container restart.
package appconfig

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mrt187/EmbyInsights/internal/secretbox"
)

// ServiceSetting is the on/off + connection details for one optional
// third-party integration. APIKey is always the decrypted plaintext when
// read from Store.Get, and the plaintext to encrypt when passed to
// Store.Update.
type ServiceSetting struct {
	Enabled bool
	BaseURL string
	APIKey  string
}

type Settings struct {
	EmbyDeviceID        string
	NewForYouLibraryIDs []string
	WatchedLibraryIDs   []string
	Seerr               ServiceSetting
	Radarr              ServiceSetting
	Sonarr              ServiceSetting
	TMDB                ServiceSetting
	OMDB                ServiceSetting
	Tracearr            ServiceSetting
	ComingSoonRegion    string
	ComingSoonDaysAhead int
	// Language is the global UI language of the frontend ("de" or "en").
	// Unlike ComingSoonRegion it never influences which metadata is fetched
	// from Radarr/Sonarr/TMDB — it only switches the interface chrome.
	Language string
}

// EnabledBaseURL returns the setting's base URL only when enabled, otherwise
// empty. Radarr/Sonarr/Seerr client constructors already treat an empty URL
// as "not configured", so this is the sole seam needed to let the admin UI
// toggle a fully-configured integration off without erasing its saved values.
func (setting ServiceSetting) EnabledBaseURL() string {
	if !setting.Enabled {
		return ""
	}
	return setting.BaseURL
}

func (setting ServiceSetting) EnabledAPIKey() string {
	if !setting.Enabled {
		return ""
	}
	return setting.APIKey
}

type Store struct {
	pool *pgxpool.Pool
	box  *secretbox.Box
}

func NewStore(pool *pgxpool.Pool, box *secretbox.Box) *Store {
	return &Store{pool: pool, box: box}
}

// Get reads the singleton app_config row, decrypting API keys for backend
// use. Callers must never forward the decrypted Settings to the browser.
func (store *Store) Get(ctx context.Context) (Settings, error) {
	if err := store.ensureRow(ctx); err != nil {
		return Settings{}, err
	}

	var (
		settings                        Settings
		seerrKeyCipher, radarrKeyCipher string
		sonarrKeyCipher, tmdbKeyCipher  string
		omdbKeyCipher                   string
		tracearrKeyCipher               string
	)
	err := store.pool.QueryRow(ctx, `
		SELECT emby_device_id, new_for_you_library_ids, watched_library_ids,
			seerr_enabled, seerr_base_url, seerr_api_key_encrypted,
			radarr_enabled, radarr_base_url, radarr_api_key_encrypted,
			sonarr_enabled, sonarr_base_url, sonarr_api_key_encrypted,
			tmdb_enabled, tmdb_api_key_encrypted,
			omdb_enabled, omdb_api_key_encrypted,
			tracearr_enabled, tracearr_base_url, tracearr_api_key_encrypted,
			comingsoon_region, comingsoon_days_ahead, language
		FROM app_config WHERE id = 1
	`).Scan(
		&settings.EmbyDeviceID, &settings.NewForYouLibraryIDs, &settings.WatchedLibraryIDs,
		&settings.Seerr.Enabled, &settings.Seerr.BaseURL, &seerrKeyCipher,
		&settings.Radarr.Enabled, &settings.Radarr.BaseURL, &radarrKeyCipher,
		&settings.Sonarr.Enabled, &settings.Sonarr.BaseURL, &sonarrKeyCipher,
		&settings.TMDB.Enabled, &tmdbKeyCipher,
		&settings.OMDB.Enabled, &omdbKeyCipher,
		&settings.Tracearr.Enabled, &settings.Tracearr.BaseURL, &tracearrKeyCipher,
		&settings.ComingSoonRegion, &settings.ComingSoonDaysAhead, &settings.Language,
	)
	if err != nil {
		return Settings{}, fmt.Errorf("read app_config: %w", err)
	}

	for _, pair := range []struct {
		cipher string
		out    *string
	}{
		{seerrKeyCipher, &settings.Seerr.APIKey},
		{radarrKeyCipher, &settings.Radarr.APIKey},
		{sonarrKeyCipher, &settings.Sonarr.APIKey},
		{tmdbKeyCipher, &settings.TMDB.APIKey},
		{omdbKeyCipher, &settings.OMDB.APIKey},
		{tracearrKeyCipher, &settings.Tracearr.APIKey},
	} {
		plaintext, err := store.box.Decrypt(pair.cipher)
		if err != nil {
			return Settings{}, fmt.Errorf("decrypt stored API key: %w", err)
		}
		*pair.out = plaintext
	}

	return settings, nil
}

// Update persists new settings. An empty APIKey on a service means "keep the
// currently stored key" — the admin UI never has the plaintext to resend,
// so it only sends a new key when the operator actually typed one.
func (store *Store) Update(ctx context.Context, settings Settings) error {
	if err := store.ensureRow(ctx); err != nil {
		return err
	}

	current, err := store.Get(ctx)
	if err != nil {
		return err
	}
	if settings.Seerr.APIKey == "" {
		settings.Seerr.APIKey = current.Seerr.APIKey
	}
	if settings.Radarr.APIKey == "" {
		settings.Radarr.APIKey = current.Radarr.APIKey
	}
	if settings.Sonarr.APIKey == "" {
		settings.Sonarr.APIKey = current.Sonarr.APIKey
	}
	if settings.TMDB.APIKey == "" {
		settings.TMDB.APIKey = current.TMDB.APIKey
	}
	if settings.OMDB.APIKey == "" {
		settings.OMDB.APIKey = current.OMDB.APIKey
	}
	if settings.Tracearr.APIKey == "" {
		settings.Tracearr.APIKey = current.Tracearr.APIKey
	}

	seerrKeyCipher, err := store.box.Encrypt(settings.Seerr.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt Seerr key: %w", err)
	}
	radarrKeyCipher, err := store.box.Encrypt(settings.Radarr.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt Radarr key: %w", err)
	}
	sonarrKeyCipher, err := store.box.Encrypt(settings.Sonarr.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt Sonarr key: %w", err)
	}
	tmdbKeyCipher, err := store.box.Encrypt(settings.TMDB.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt TMDB key: %w", err)
	}
	omdbKeyCipher, err := store.box.Encrypt(settings.OMDB.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt OMDB key: %w", err)
	}
	tracearrKeyCipher, err := store.box.Encrypt(settings.Tracearr.APIKey)
	if err != nil {
		return fmt.Errorf("encrypt Tracearr key: %w", err)
	}

	_, err = store.pool.Exec(ctx, `
		UPDATE app_config SET
			new_for_you_library_ids = $1, watched_library_ids = $2,
			seerr_enabled = $3, seerr_base_url = $4, seerr_api_key_encrypted = $5,
			radarr_enabled = $6, radarr_base_url = $7, radarr_api_key_encrypted = $8,
			sonarr_enabled = $9, sonarr_base_url = $10, sonarr_api_key_encrypted = $11,
			tmdb_enabled = $12, tmdb_api_key_encrypted = $13,
			omdb_enabled = $14, omdb_api_key_encrypted = $15,
			tracearr_enabled = $16, tracearr_base_url = $17, tracearr_api_key_encrypted = $18,
			comingsoon_region = $19, comingsoon_days_ahead = $20,
			language = $21,
			updated_at = now()
		WHERE id = 1
	`,
		settings.NewForYouLibraryIDs, settings.WatchedLibraryIDs,
		settings.Seerr.Enabled, settings.Seerr.BaseURL, seerrKeyCipher,
		settings.Radarr.Enabled, settings.Radarr.BaseURL, radarrKeyCipher,
		settings.Sonarr.Enabled, settings.Sonarr.BaseURL, sonarrKeyCipher,
		settings.TMDB.Enabled, tmdbKeyCipher,
		settings.OMDB.Enabled, omdbKeyCipher,
		settings.Tracearr.Enabled, settings.Tracearr.BaseURL, tracearrKeyCipher,
		settings.ComingSoonRegion, settings.ComingSoonDaysAhead,
		valueOr(settings.Language, "en"),
	)
	if err != nil {
		return fmt.Errorf("update app_config: %w", err)
	}
	return nil
}

// EnsureDeviceID returns the persisted Emby device id, generating and
// storing a new one on first use. Replaces the operator-supplied
// EMBY_DEVICE_ID env var.
func (store *Store) EnsureDeviceID(ctx context.Context) (string, error) {
	if err := store.ensureRow(ctx); err != nil {
		return "", err
	}

	var deviceID string
	if err := store.pool.QueryRow(ctx, `SELECT emby_device_id FROM app_config WHERE id = 1`).Scan(&deviceID); err != nil {
		return "", fmt.Errorf("read emby_device_id: %w", err)
	}
	if deviceID != "" {
		return deviceID, nil
	}

	deviceID = newUUID()
	if _, err := store.pool.Exec(ctx, `UPDATE app_config SET emby_device_id = $1, updated_at = now() WHERE id = 1 AND emby_device_id = ''`, deviceID); err != nil {
		return "", fmt.Errorf("persist emby_device_id: %w", err)
	}
	// Someone else may have set it concurrently between the SELECT and this
	// UPDATE; re-read to return whichever id actually won.
	if err := store.pool.QueryRow(ctx, `SELECT emby_device_id FROM app_config WHERE id = 1`).Scan(&deviceID); err != nil {
		return "", fmt.Errorf("read emby_device_id after generation: %w", err)
	}
	return deviceID, nil
}

// ClaimAdminOwner atomically assigns candidateUserID as the Emby Insights
// admin if nobody holds that role yet, and always returns the true owner
// (whoever won), so concurrent first logins never produce two admins.
func (store *Store) ClaimAdminOwner(ctx context.Context, candidateUserID string) (string, error) {
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO admin_owner (id, emby_user_id) VALUES (1, $1)
		ON CONFLICT (id) DO NOTHING
	`, candidateUserID); err != nil {
		return "", fmt.Errorf("claim admin owner: %w", err)
	}
	return store.CurrentAdminOwner(ctx)
}

// CurrentAdminOwner reads the assigned admin without attempting to claim the
// role, for per-request admin checks outside the login flow.
func (store *Store) CurrentAdminOwner(ctx context.Context) (string, error) {
	var ownerID string
	err := store.pool.QueryRow(ctx, `SELECT emby_user_id FROM admin_owner WHERE id = 1`).Scan(&ownerID)
	if err != nil {
		// No admin_owner row yet is the normal pre-first-login state, not a
		// failure — callers treat "" as "nobody is admin yet".
		return "", nil
	}
	return ownerID, nil
}

// SeedFromEnvIfEmpty runs once at boot, before anything else touches
// app_config/admin_owner. If those tables are still completely unset and the
// legacy .env variables they replace are present, it copies them over once,
// so upgrading an existing install never silently loses a working setup.
func (store *Store) SeedFromEnvIfEmpty(ctx context.Context) error {
	if err := store.ensureRow(ctx); err != nil {
		return err
	}

	var configured bool
	if err := store.pool.QueryRow(ctx, `
		SELECT seerr_enabled OR radarr_enabled OR sonarr_enabled OR tmdb_enabled
			OR array_length(new_for_you_library_ids, 1) IS NOT NULL
			OR array_length(watched_library_ids, 1) IS NOT NULL
		FROM app_config WHERE id = 1
	`).Scan(&configured); err != nil {
		return fmt.Errorf("check existing app_config: %w", err)
	}
	if configured {
		return nil
	}

	settings := Settings{
		NewForYouLibraryIDs: splitList(os.Getenv("EMBY_NEW_FOR_YOU_LIBRARY_IDS")),
		WatchedLibraryIDs:   splitList(os.Getenv("EMBY_WATCHED_LIBRARY_IDS")),
		Seerr:               envService("SEERR_URL", "SEERR_API_KEY"),
		Radarr:              envService("RADARR_URL", "RADARR_API_KEY"),
		Sonarr:              envService("SONARR_URL", "SONARR_API_KEY"),
		TMDB:                ServiceSetting{Enabled: os.Getenv("TMDB_API_KEY") != "", APIKey: os.Getenv("TMDB_API_KEY")},
		ComingSoonRegion:    valueOr(os.Getenv("COMINGSOON_REGION"), "DE"),
		ComingSoonDaysAhead: intValueOr(os.Getenv("COMINGSOON_DAYS_AHEAD"), 28),
		Language:            valueOr(os.Getenv("UI_LANGUAGE"), "en"),
	}
	if !settings.Seerr.Enabled && !settings.Radarr.Enabled && !settings.Sonarr.Enabled &&
		!settings.TMDB.Enabled && settings.NewForYouLibraryIDs == nil && settings.WatchedLibraryIDs == nil {
		return nil
	}
	if err := store.Update(ctx, settings); err != nil {
		return fmt.Errorf("seed app_config from env: %w", err)
	}

	if legacyAdmin := strings.TrimSpace(os.Getenv("ADMIN_EMBY_USER_ID")); legacyAdmin != "" {
		if _, err := store.ClaimAdminOwner(ctx, legacyAdmin); err != nil {
			return fmt.Errorf("seed admin_owner from env: %w", err)
		}
	}
	return nil
}

func (store *Store) ensureRow(ctx context.Context) error {
	_, err := store.pool.Exec(ctx, `INSERT INTO app_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("ensure app_config row: %w", err)
	}
	return nil
}

func envService(urlVar, keyVar string) ServiceSetting {
	url := strings.TrimSpace(os.Getenv(urlVar))
	key := strings.TrimSpace(os.Getenv(keyVar))
	return ServiceSetting{Enabled: url != "" && key != "", BaseURL: url, APIKey: key}
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func intValueOr(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

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

// newUUID generates a random UUIDv4 without pulling in an external
// dependency the module doesn't otherwise need.
func newUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		panic(fmt.Errorf("generate device id: %w", err))
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}
