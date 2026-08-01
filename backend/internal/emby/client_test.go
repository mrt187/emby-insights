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
		if string(body) != `{"Username":"testuser","Pw":"secret"}` {
			t.Fatalf("body = %s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"User":{"Id":"user-1","Name":"Test User"},"AccessToken":"token","ServerId":"server-1"}`))
	}))
	defer testServer.Close()
	identity, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").Authenticate(context.Background(), Credentials{Username: "testuser", Password: "secret"})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if identity.UserID != "user-1" || identity.DisplayName != "Test User" || identity.AccessToken != "token" {
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
		_, _ = writer.Write([]byte(`{"WatchSeconds":3600,"PreviousWatchSeconds":1800,"CompletedMovies":2,"CompletedSeries":1,"FavouriteGenre":"Drama","PeriodStartsAt":"2026-07-27T00:00:00Z","PeriodEndsAt":"2026-07-28T12:00:00Z"}`))
	}))
	defer testServer.Close()

	statistics, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").PersonalWatchTime(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("PersonalWatchTime() error = %v", err)
	}
	if statistics.WatchSeconds != 3600 || statistics.PreviousWatchSeconds != 1800 {
		t.Fatalf("statistics = %#v", statistics)
	}
	if statistics.CompletedMovies != 2 || statistics.CompletedSeries != 1 || statistics.FavouriteGenre != "Drama" {
		t.Fatalf("statistics = %#v", statistics)
	}
}

func TestDeviceWatchTimesUsesAdminAPIKey(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/EmbyInsights/PersonalStats/Devices" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		if request.URL.Query().Get("UserId") != "user-1" || request.URL.Query().Get("Period") != "month" {
			t.Fatalf("query = %s", request.URL.RawQuery)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"deviceName":"FireTV 4K","watchSeconds":3600},{"deviceName":"iPhone","watchSeconds":1200}]`))
	}))
	defer testServer.Close()

	devices, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").DeviceWatchTimes(context.Background(), "user-1", "month")
	if err != nil {
		t.Fatalf("DeviceWatchTimes() error = %v", err)
	}
	if len(devices) != 2 || devices[0].DeviceName != "FireTV 4K" || devices[0].WatchSeconds != 3600 {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestHourWatchTimesUsesAdminAPIKey(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/EmbyInsights/PersonalStats/Hours" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"hour":21,"watchSeconds":7200}]`))
	}))
	defer testServer.Close()

	hours, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").HourWatchTimes(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("HourWatchTimes() error = %v", err)
	}
	if len(hours) != 1 || hours[0].Hour != 21 || hours[0].WatchSeconds != 7200 {
		t.Fatalf("hours = %#v", hours)
	}
}

func TestWeekdayWatchTimesUsesAdminAPIKey(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/EmbyInsights/PersonalStats/Weekdays" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[{"weekday":0,"watchSeconds":1800}]`))
	}))
	defer testServer.Close()

	weekdays, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").WeekdayWatchTimes(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("WeekdayWatchTimes() error = %v", err)
	}
	if len(weekdays) != 1 || weekdays[0].Weekday != 0 || weekdays[0].WatchSeconds != 1800 {
		t.Fatalf("weekdays = %#v", weekdays)
	}
}

func TestLongestSessionReportsNotFoundWhenEmpty(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/EmbyInsights/PersonalStats/LongestSession" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"itemName":"","watchSeconds":0,"startedAt":"0001-01-01T00:00:00+00:00"}`))
	}))
	defer testServer.Close()

	session, found, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").LongestSession(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("LongestSession() error = %v", err)
	}
	if found {
		t.Fatalf("found = true, want false for an empty period, session = %#v", session)
	}
}

func TestLongestSessionReadsRealSession(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"itemName":"Shelter","watchSeconds":12531,"startedAt":"2026-03-29T00:49:18Z"}`))
	}))
	defer testServer.Close()

	session, found, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").LongestSession(context.Background(), "user-1", "year")
	if err != nil {
		t.Fatalf("LongestSession() error = %v", err)
	}
	if !found || session.ItemName != "Shelter" || session.WatchSeconds != 12531 {
		t.Fatalf("found = %v, session = %#v", found, session)
	}
}

func TestMostActiveDayReportsNotFoundWhenEmpty(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/EmbyInsights/PersonalStats/MostActiveDay" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"date":"0001-01-01T00:00:00+00:00","watchSeconds":0}`))
	}))
	defer testServer.Close()

	day, found, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").MostActiveDay(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("MostActiveDay() error = %v", err)
	}
	if found {
		t.Fatalf("found = true, want false for an empty period, day = %#v", day)
	}
}

func TestMostActiveDayReadsRealDay(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"date":"2026-02-01T00:00:00Z","watchSeconds":31924}`))
	}))
	defer testServer.Close()

	day, found, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").MostActiveDay(context.Background(), "user-1", "year")
	if err != nil {
		t.Fatalf("MostActiveDay() error = %v", err)
	}
	if !found || day.WatchSeconds != 31924 {
		t.Fatalf("found = %v, day = %#v", found, day)
	}
}
