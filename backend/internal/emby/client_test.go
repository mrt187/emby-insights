package emby

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthenticateSendsEmbyRequest(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/AuthenticateByName" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if !strings.Contains(request.Header.Get("X-Emby-Authorization"), "DeviceId=\"dashboard-device\"") {
			t.Fatalf("missing device id header: %q", request.Header.Get("X-Emby-Authorization"))
		}
		body, _ := io.ReadAll(request.Body)
		if string(body) != `{"Username":"thomas","Pw":"secret"}` {
			t.Fatalf("body = %s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"User":{"Id":"user-1","Name":"Thomas"},"AccessToken":"token","ServerId":"server-1"}`))
	}))
	defer testServer.Close()
	identity, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").Authenticate(context.Background(), Credentials{Username: "thomas", Password: "secret"})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if identity.UserID != "user-1" || identity.DisplayName != "Thomas" || identity.AccessToken != "token" {
		t.Fatalf("identity = %#v", identity)
	}
}

func TestAuthenticateRejectsInvalidCredentials(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusUnauthorized) }))
	defer testServer.Close()
	_, err := NewClient(testServer.URL, "dashboard-device", "admin-key").Authenticate(context.Background(), Credentials{})
	if err != ErrInvalidCredentials {
		t.Fatalf("Authenticate() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestPersonalWatchTimeUsesAdminAPIKey(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/EmbyInsights/PersonalStats" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		if request.URL.Query().Get("UserId") != "user-1" || request.URL.Query().Get("Period") != "week" {
			t.Fatalf("query = %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"WatchSeconds":3600,"PreviousWatchSeconds":1800,"PeriodStartsAt":"2026-07-27T00:00:00Z","PeriodEndsAt":"2026-07-28T12:00:00Z"}`))
	}))
	defer testServer.Close()

	statistics, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").PersonalWatchTime(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("PersonalWatchTime() error = %v", err)
	}
	if statistics.WatchSeconds != 3600 || statistics.PreviousWatchSeconds != 1800 {
		t.Fatalf("statistics = %#v", statistics)
	}
}
