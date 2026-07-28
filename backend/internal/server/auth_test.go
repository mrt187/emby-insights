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
