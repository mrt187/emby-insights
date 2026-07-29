package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWatchedMoviesFiltersToCurrentWeek(t *testing.T) {
	now := time.Now().UTC()
	thisWeek := now.Add(-6 * time.Hour).Format(time.RFC3339)
	lastMonth := now.AddDate(0, -1, 0).Format(time.RFC3339)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1/Items" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.URL.Query().Get("IncludeItemTypes") != "Movie" {
			t.Fatalf("IncludeItemTypes = %q", request.URL.Query().Get("IncludeItemTypes"))
		}
		if request.URL.Query().Get("Filters") != "IsPlayed" {
			t.Fatalf("Filters = %q", request.URL.Query().Get("Filters"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Items":[
			{"Id":"1","Name":"Dune","Genres":["Science Fiction"],"ImageTags":{"Primary":"tag-1"},"UserData":{"LastPlayedDate":"` + thisWeek + `"}},
			{"Id":"2","Name":"Old Movie","Genres":["Drama"],"UserData":{"LastPlayedDate":"` + lastMonth + `"}},
			{"Id":"3","Name":"NeverPlayed"}
		]}`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").WatchedMovies(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("WatchedMovies() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Dune" {
		t.Fatalf("items = %#v", items)
	}
	if len(items[0].Genres) != 1 || items[0].Genres[0] != "Science Fiction" {
		t.Fatalf("genres = %#v", items[0].Genres)
	}
}

func TestWatchedSeriesUsesSeriesItemType(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("IncludeItemTypes") != "Series" {
			t.Fatalf("IncludeItemTypes = %q", request.URL.Query().Get("IncludeItemTypes"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Items":[{"Id":"1","Name":"Severance","UserData":{"LastPlayedDate":"` + now + `"}}]}`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").WatchedSeries(context.Background(), "user-1", "week")
	if err != nil {
		t.Fatalf("WatchedSeries() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Severance" {
		t.Fatalf("items = %#v", items)
	}
}

func TestPeriodRangeMirrorsPlugin(t *testing.T) {
	monday := time.Date(2026, 7, 29, 15, 0, 0, 0, time.UTC) // a Wednesday
	from, to, err := periodRange("week", monday)
	if err != nil {
		t.Fatalf("periodRange() error = %v", err)
	}
	wantFrom := time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC) // preceding Monday
	if !from.Equal(wantFrom) {
		t.Fatalf("from = %v, want %v", from, wantFrom)
	}
	if !to.Equal(monday) {
		t.Fatalf("to = %v, want %v", to, monday)
	}
}
