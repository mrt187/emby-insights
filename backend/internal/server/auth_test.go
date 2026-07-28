package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/emby"
)

type fakeAuthenticator struct {
	identity emby.Identity
	err      error
}

func (auth fakeAuthenticator) Authenticate(context.Context, emby.Credentials) (emby.Identity, error) {
	return auth.identity, auth.err
}

type fakeStatisticsReader struct {
	statistics emby.PersonalWatchTime
	userID     string
	period     string
}

func (reader *fakeStatisticsReader) PersonalWatchTime(_ context.Context, userID, period string) (emby.PersonalWatchTime, error) {
	reader.userID = userID
	reader.period = period
	return reader.statistics, nil
}

type memorySessionStore struct {
	identity emby.Identity
	deleted  bool
}

func (store *memorySessionStore) Create(_ context.Context, identity emby.Identity) (string, error) {
	store.identity = identity
	return "session-id", nil
}

func (store *memorySessionStore) Get(_ context.Context, identifier string) (emby.Identity, error) {
	if identifier != "session-id" {
		return emby.Identity{}, errors.New("unknown session")
	}
	return store.identity, nil
}

func (store *memorySessionStore) Delete(_ context.Context, _ string) error {
	store.deleted = true
	return nil
}

func TestLoginCreatesSessionWithoutExposingEmbyToken(t *testing.T) {
	store := &memorySessionStore{}
	app := &App{
		authenticator: fakeAuthenticator{identity: emby.Identity{UserID: "user-1", DisplayName: "Thomas", AccessToken: "secret-token"}},
		sessions:      store,
		cookieSecure:  true,
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(`{"username":"thomas","password":"password"}`))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-token") {
		t.Fatal("login response exposed Emby access token")
	}
	cookie := recorder.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.Value != "session-id" {
		t.Fatalf("cookie = %#v", cookie)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	app := &App{authenticator: fakeAuthenticator{err: emby.ErrInvalidCredentials}, sessions: &memorySessionStore{}}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(`{"username":"thomas","password":"wrong"}`))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestStatsUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1", DisplayName: "Thomas", AccessToken: "secret-token"}}
	statistics := &fakeStatisticsReader{statistics: emby.PersonalWatchTime{WatchSeconds: 3600, PreviousWatchSeconds: 1800}}
	app := &App{sessions: store, statistics: statistics}
	request := httptest.NewRequest(http.MethodGet, "/api/stats?period=month", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if statistics.userID != "user-1" || statistics.period != "month" {
		t.Fatalf("statistics request = user %q, period %q", statistics.userID, statistics.period)
	}
	if strings.Contains(recorder.Body.String(), "secret-token") {
		t.Fatal("statistics response exposed Emby access token")
	}
}
