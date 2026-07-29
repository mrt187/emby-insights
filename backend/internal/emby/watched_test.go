package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWatchedMoviesMergesLibrariesAndSortsByLastPlayed(t *testing.T) {
	recent := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	old := time.Now().UTC().AddDate(-1, 0, 0).Format(time.RFC3339)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/emby/Users/user-1/Items" && request.URL.Query().Get("ParentId") == "3":
			if request.URL.Query().Get("IncludeItemTypes") != "Movie" {
				t.Fatalf("IncludeItemTypes = %q", request.URL.Query().Get("IncludeItemTypes"))
			}
			if request.URL.Query().Get("Filters") != "IsPlayed" {
				t.Fatalf("Filters = %q", request.URL.Query().Get("Filters"))
			}
			_, _ = writer.Write([]byte(`{"Items":[{"Id":"2","Name":"Old Movie","Genres":["Drama"]}]}`))
		case request.URL.Path == "/emby/Users/user-1/Items" && request.URL.Query().Get("ParentId") == "123857":
			_, _ = writer.Write([]byte(`{"Items":[{"Id":"1","Name":"Dune","Genres":["Science Fiction"],"ImageTags":{"Primary":"tag-1"}}]}`))
		case request.URL.Path == "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"` + recent + `"}}`))
		case request.URL.Path == "/emby/Users/user-1/Items/2":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"` + old + `"}}`))
		default:
			t.Fatalf("unexpected request %q", request.URL.String())
		}
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").WatchedMovies(context.Background(), "user-1", []string{"3", "123857"})
	if err != nil {
		t.Fatalf("WatchedMovies() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %#v, want both movies from both libraries", items)
	}
	if items[0].Title != "Dune" || len(items[0].Genres) != 1 || items[0].Genres[0] != "Science Fiction" {
		t.Fatalf("items[0] (most recently played) = %#v", items[0])
	}
	if items[1].Title != "Old Movie" {
		t.Fatalf("items[1] = %#v", items[1])
	}
}

func TestWatchedSeriesUsesSeriesItemType(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/emby/Users/user-1/Items":
			if request.URL.Query().Get("IncludeItemTypes") != "Series" {
				t.Fatalf("IncludeItemTypes = %q", request.URL.Query().Get("IncludeItemTypes"))
			}
			if request.URL.Query().Get("ParentId") != "5" {
				t.Fatalf("ParentId = %q", request.URL.Query().Get("ParentId"))
			}
			_, _ = writer.Write([]byte(`{"Items":[{"Id":"1","Name":"Severance"}]}`))
		case "/emby/Users/user-1/Items/1":
			_, _ = writer.Write([]byte(`{"UserData":{"LastPlayedDate":"` + now + `"}}`))
		default:
			t.Fatalf("unexpected path %q", request.URL.Path)
		}
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").WatchedSeries(context.Background(), "user-1", []string{"5"})
	if err != nil {
		t.Fatalf("WatchedSeries() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Severance" {
		t.Fatalf("items = %#v", items)
	}
}

func TestWatchedMoviesWithoutLibraryIDsSkipsRequest(t *testing.T) {
	items, err := NewClient("http://unused", "device", "key").WatchedMovies(context.Background(), "user-1", nil)
	if err != nil || items != nil {
		t.Fatalf("WatchedMovies() = %#v, %v, want nil, nil", items, err)
	}
}
