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
func (store *fakeConfigStore) Update(_ context.Context, settings appconfig.Settings) error {
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
