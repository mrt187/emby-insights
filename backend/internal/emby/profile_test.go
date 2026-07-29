package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserProfileReadsMemberSinceAndLastActiveDate(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("X-Emby-Token = %q", request.Header.Get("X-Emby-Token"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"DateCreated":"2026-01-14T08:00:48.6590903Z",
			"LastActivityDate":"2026-07-29T13:59:59.4594171Z",
			"LastLoginDate":"2026-07-29T10:18:45.2115017Z"
		}`))
	}))
	defer testServer.Close()

	profile, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").UserProfile(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("UserProfile() error = %v", err)
	}
	if profile.MemberSince != "2026-01-14T08:00:48.6590903Z" {
		t.Fatalf("MemberSince = %q", profile.MemberSince)
	}
	if profile.LastActiveDate != "2026-07-29T13:59:59.4594171Z" {
		t.Fatalf("LastActiveDate = %q", profile.LastActiveDate)
	}
	if profile.LastLoginDate != "2026-07-29T10:18:45.2115017Z" {
		t.Fatalf("LastLoginDate = %q", profile.LastLoginDate)
	}
}
