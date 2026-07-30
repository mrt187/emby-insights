package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/appconfig"
	"github.com/mrt187/EmbyInsights/internal/emby"
)

// TestValidateServiceURLAllowsPrivateLAN guards against the SSRF check
// blocking the primary use case: Seerr/Radarr/Sonarr running on the
// operator's own home network. This regressed once already — see v0.8.57.
func TestValidateServiceURLAllowsPrivateLAN(t *testing.T) {
	for _, url := range []string{
		"http://10.0.0.2:5055",
		"http://192.168.1.50:7878",
		"http://172.16.0.5:8989",
	} {
		if err := validateServiceURL(url); err != nil {
			t.Fatalf("validateServiceURL(%q) error = %v, want nil (private LAN must be allowed)", url, err)
		}
	}
}

func TestValidateServiceURLRejectsLoopbackAndLinkLocal(t *testing.T) {
	for _, url := range []string{
		"http://localhost:5055",
		"http://127.0.0.1:5055",
		"http://[::1]:5055",
		"http://169.254.169.254/latest/meta-data",
	} {
		if err := validateServiceURL(url); err == nil {
			t.Fatalf("validateServiceURL(%q) error = nil, want rejection", url)
		}
	}
}

// fakeConfigStore is an in-memory stand-in for appconfig.Store, so admin
// gating and the features matrix can be unit-tested without a real Postgres
// instance — mirrors the atomic "first claim wins" contract the real store
// guarantees via its unique-constraint INSERT ... ON CONFLICT DO NOTHING.
type fakeConfigStore struct {
	settings appconfig.Settings
	ownerID  string
}

func (store *fakeConfigStore) Get(context.Context) (appconfig.Settings, error) {
	return store.settings, nil
}

// Update mirrors appconfig.Store.Update's real merge behavior (an empty
// APIKey keeps the previously stored one) so tests here can catch bugs in
// how callers use the result — the real regression this guards against is
// applySettings rebuilding the live Seerr/Radarr/Sonarr/TMDB clients from
// its own unmerged input instead of re-reading what was actually persisted.
func (store *fakeConfigStore) Update(_ context.Context, settings appconfig.Settings) error {
	if settings.Seerr.APIKey == "" {
		settings.Seerr.APIKey = store.settings.Seerr.APIKey
	}
	if settings.Radarr.APIKey == "" {
		settings.Radarr.APIKey = store.settings.Radarr.APIKey
	}
	if settings.Sonarr.APIKey == "" {
		settings.Sonarr.APIKey = store.settings.Sonarr.APIKey
	}
	if settings.TMDB.APIKey == "" {
		settings.TMDB.APIKey = store.settings.TMDB.APIKey
	}
	store.settings = settings
	return nil
}
func (store *fakeConfigStore) EnsureDeviceID(context.Context) (string, error) { return "device-1", nil }
func (store *fakeConfigStore) ClaimAdminOwner(_ context.Context, candidateUserID string) (string, error) {
	if store.ownerID == "" {
		store.ownerID = candidateUserID
	}
	return store.ownerID, nil
}
func (store *fakeConfigStore) CurrentAdminOwner(context.Context) (string, error) {
	return store.ownerID, nil
}
func (store *fakeConfigStore) SeedFromEnvIfEmpty(context.Context) error { return nil }

func loggedInApp(t *testing.T, configStore ConfigStore, identity emby.Identity) (*App, *http.Cookie) {
	t.Helper()
	sessions := &memorySessionStore{identity: identity}
	app := &App{sessions: sessions, appconfig: configStore}
	return app, &http.Cookie{Name: sessionCookieName, Value: "session-id"}
}

func TestRequireAdminRejectsBeforeAnyoneHasClaimedOwnership(t *testing.T) {
	configStore := &fakeConfigStore{}
	app, cookie := loggedInApp(t, configStore, emby.Identity{UserID: "user-1", DisplayName: "Alice"})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 before any admin has been claimed", recorder.Code)
	}
}

func TestRequireAdminAllowsTheClaimedOwner(t *testing.T) {
	configStore := &fakeConfigStore{ownerID: "user-1"}
	app, cookie := loggedInApp(t, configStore, emby.Identity{UserID: "user-1", DisplayName: "Alice"})
	app.directory = fakeUserDirectory{}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s, want 200 for the claimed owner", recorder.Code, recorder.Body.String())
	}
}

func TestRequireAdminRejectsANonOwner(t *testing.T) {
	configStore := &fakeConfigStore{ownerID: "user-1"}
	app, cookie := loggedInApp(t, configStore, emby.Identity{UserID: "user-2", DisplayName: "Bob"})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/users", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 for a logged-in user who isn't the admin", recorder.Code)
	}
}

type fakeUserDirectory struct{}

func (fakeUserDirectory) Users(context.Context) ([]emby.EmbyUser, error) {
	return []emby.EmbyUser{{ID: "user-1", Name: "Alice"}, {ID: "user-2", Name: "Bob"}}, nil
}

func TestMeReportsFeaturesFromSettings(t *testing.T) {
	configStore := &fakeConfigStore{
		ownerID: "user-1",
		settings: appconfig.Settings{
			Seerr:  appconfig.ServiceSetting{Enabled: true},
			Radarr: appconfig.ServiceSetting{Enabled: true},
			Sonarr: appconfig.ServiceSetting{Enabled: false},
		},
	}
	app, cookie := loggedInApp(t, configStore, emby.Identity{UserID: "user-1", DisplayName: "Alice"})

	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, want := range []string{`"isAdmin":true`, `"requests":true`, `"movieDates":true`, `"seriesDates":false`, `"upcoming":true`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body = %s, want to contain %q", body, want)
		}
	}
}

func TestMeReportsUpcomingFalseWhenNeitherRadarrNorSonarrEnabled(t *testing.T) {
	configStore := &fakeConfigStore{ownerID: "user-1"}
	app, cookie := loggedInApp(t, configStore, emby.Identity{UserID: "user-1", DisplayName: "Alice"})

	request := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if !strings.Contains(recorder.Body.String(), `"upcoming":false`) {
		t.Fatalf("body = %s, want upcoming:false when Radarr and Sonarr are both disabled", recorder.Body.String())
	}
}

func TestLoginClaimsFirstAdminAtomically(t *testing.T) {
	configStore := &fakeConfigStore{}
	app := &App{
		authenticator: fakeAuthenticator{identity: emby.Identity{UserID: "user-1", DisplayName: "Alice"}},
		sessions:      &memorySessionStore{},
		appconfig:     configStore,
		loginLimiters: make(map[string]*ipRateLimiter),
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(`{"username":"alice","password":"password"}`))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"isAdmin":true`) {
		t.Fatalf("body = %s, want the first login to become admin", recorder.Body.String())
	}
	if configStore.ownerID != "user-1" {
		t.Fatalf("ownerID = %q, want %q", configStore.ownerID, "user-1")
	}
}

func TestLoginDoesNotReclaimAdminForALaterUser(t *testing.T) {
	configStore := &fakeConfigStore{ownerID: "user-1"}
	app := &App{
		authenticator: fakeAuthenticator{identity: emby.Identity{UserID: "user-2", DisplayName: "Bob"}},
		sessions:      &memorySessionStore{},
		appconfig:     configStore,
		loginLimiters: make(map[string]*ipRateLimiter),
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(`{"username":"bob","password":"password"}`))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"isAdmin":true`) {
		t.Fatalf("body = %s, want a later login to not become admin", recorder.Body.String())
	}
	if configStore.ownerID != "user-1" {
		t.Fatalf("ownerID = %q, want the original owner %q to be kept", configStore.ownerID, "user-1")
	}
}

// TestApplySettingsKeepsLiveSeerrClientAfterResavingWithoutTheAPIKey guards
// against a real production bug: applySettings used to rebuild the live
// Seerr client from its own (possibly still-empty) input instead of what
// Update actually persisted. Saving Verwaltung settings a second time
// without retyping the API key — the normal flow, since the field always
// starts blank — silently disabled Seerr at runtime while the database and
// the admin GET view still showed it fully configured.
func TestApplySettingsKeepsLiveSeerrClientAfterResavingWithoutTheAPIKey(t *testing.T) {
	configStore := &fakeConfigStore{}
	app := &App{appconfig: configStore, live: &liveConfig{}}
	ctx := context.Background()

	if err := app.applySettings(ctx, appconfig.Settings{
		Seerr: appconfig.ServiceSetting{Enabled: true, BaseURL: "http://seerr.local", APIKey: "real-key"},
	}); err != nil {
		t.Fatalf("applySettings() first save error = %v", err)
	}
	if seerrClient, _, _ := app.live.current(); seerrClient == nil {
		t.Fatal("live Seerr client is nil after the first save with a real key")
	}

	// Simulates clicking "Speichern" again without retyping the key — the
	// field is empty because the GET response never returns the plaintext.
	if err := app.applySettings(ctx, appconfig.Settings{
		Seerr: appconfig.ServiceSetting{Enabled: true, BaseURL: "http://seerr.local", APIKey: ""},
	}); err != nil {
		t.Fatalf("applySettings() second save error = %v", err)
	}
	if seerrClient, _, _ := app.live.current(); seerrClient == nil {
		t.Fatal("live Seerr client became nil after resaving without retyping the API key")
	}
}

func TestAdminDebugLiveReportsSeerrConfigured(t *testing.T) {
	configStore := &fakeConfigStore{ownerID: "user-1"}
	app, cookie := loggedInApp(t, configStore, emby.Identity{UserID: "user-1", DisplayName: "Alice"})
	app.live = &liveConfig{}
	if err := app.applySettings(context.Background(), appconfig.Settings{
		Seerr: appconfig.ServiceSetting{Enabled: true, BaseURL: "http://seerr.local", APIKey: "real-key"},
	}); err != nil {
		t.Fatalf("applySettings() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/admin/debug/live", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"seerrConfigured":true`) {
		t.Fatalf("body = %s, want seerrConfigured:true", recorder.Body.String())
	}
}
