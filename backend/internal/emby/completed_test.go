package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompletedMoviesFiltersByPeriod(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	inPeriod := "2026-07-15T10:00:00Z"
	beforePeriod := "2026-06-30T23:00:00Z"

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/emby/Users/user-1/Items":
			_, _ = writer.Write([]byte(`{"Items":[{"Id":"1","Name":"Dune"},{"Id":"2","Name":"Old Movie"}]}`))
		case "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"` + inPeriod + `"}}`))
		case "/emby/Users/user-1/Items/2":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"` + beforePeriod + `"}}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "device", "key").CompletedMovies(context.Background(), "user-1", []string{"3"}, from, to)
	if err != nil {
		t.Fatalf("CompletedMovies() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Dune" {
		t.Fatalf("items = %#v, want only the movie played inside [from, to)", items)
	}
}

func TestCompletedMoviesExcludesPeriodEndBoundary(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/emby/Users/user-1/Items":
			_, _ = writer.Write([]byte(`{"Items":[{"Id":"1","Name":"Right At The Boundary"}]}`))
		case "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"2026-08-01T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "device", "key").CompletedMovies(context.Background(), "user-1", []string{"3"}, from, to)
	if err != nil {
		t.Fatalf("CompletedMovies() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %#v, want the period end boundary excluded", items)
	}
}

func TestCompletedSeriesUsesSeriesItemType(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/emby/Users/user-1/Items":
			if request.URL.Query().Get("IncludeItemTypes") != "Series" {
				t.Fatalf("IncludeItemTypes = %q", request.URL.Query().Get("IncludeItemTypes"))
			}
			_, _ = writer.Write([]byte(`{"Items":[{"Id":"1","Name":"Severance"}]}`))
		case "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"2026-06-01T00:00:00Z"}}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "device", "key").CompletedSeries(context.Background(), "user-1", []string{"5"}, from, to)
	if err != nil {
		t.Fatalf("CompletedSeries() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Severance" {
		t.Fatalf("items = %#v", items)
	}
}

func TestCompletedMoviesWithoutLibraryIDsSkipsRequest(t *testing.T) {
	items, err := NewClient("http://unused", "device", "key").CompletedMovies(context.Background(), "user-1", nil, time.Now(), time.Now())
	if err != nil || items != nil {
		t.Fatalf("CompletedMovies() = %#v, %v, want nil, nil", items, err)
	}
}

func TestPeriodBoundsMatchesPluginLogic(t *testing.T) {
	// Wednesday 2026-07-29 12:00 UTC.
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)

	weekFrom, weekTo := PeriodBounds("week", now)
	if want := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC); !weekFrom.Equal(want) {
		t.Fatalf("week from = %v, want Monday %v", weekFrom, want)
	}
	if !weekTo.Equal(now) {
		t.Fatalf("week to = %v, want now (%v)", weekTo, now)
	}

	monthFrom, _ := PeriodBounds("month", now)
	if want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC); !monthFrom.Equal(want) {
		t.Fatalf("month from = %v, want %v", monthFrom, want)
	}

	yearFrom, _ := PeriodBounds("year", now)
	if want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC); !yearFrom.Equal(want) {
		t.Fatalf("year from = %v, want %v", yearFrom, want)
	}
}
