package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewForYouFiltersToLastFourteenDays(t *testing.T) {
	recent := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	tooOld := time.Now().UTC().Add(-20 * 24 * time.Hour).Format(time.RFC3339)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1/Items/Latest" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		if request.URL.Query().Get("IsPlayed") != "false" {
			t.Fatalf("IsPlayed = %q", request.URL.Query().Get("IsPlayed"))
		}
		if request.URL.Query().Get("ParentId") != "library-1" {
			t.Fatalf("ParentId = %q", request.URL.Query().Get("ParentId"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[
			{"Id":"1","Name":"Recent","DateCreated":"` + recent + `","ImageTags":{"Primary":"tag-1"}},
			{"Id":"2","Name":"TooOld","DateCreated":"` + tooOld + `"},
			{"Id":"3","Name":"NoDate"}
		]`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").NewForYou(context.Background(), "user-1", []string{"library-1"})
	if err != nil {
		t.Fatalf("NewForYou() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Recent" {
		t.Fatalf("items = %#v", items)
	}
	if items[0].PosterURL == "" {
		t.Fatalf("expected poster URL for item with an image tag")
	}
}

func TestNewForYouIncludesNewEpisodesWithSeriesPoster(t *testing.T) {
	recent := time.Now().UTC().Add(-2 * 24 * time.Hour).Format(time.RFC3339)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.URL.Query().Get("IncludeItemTypes"); got != "Movie,Series,Episode" {
			t.Fatalf("IncludeItemTypes = %q", got)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[
			{"Id":"e1","Name":"Chapter 5","DateCreated":"` + recent + `","SeriesName":"Severance","SeriesId":"s1","SeriesPrimaryImageTag":"series-tag","ParentIndexNumber":2,"IndexNumber":5}
		]`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").NewForYou(context.Background(), "user-1", []string{"library-1"})
	if err != nil {
		t.Fatalf("NewForYou() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %#v", items)
	}
	item := items[0]
	if item.SeriesName != "Severance" || item.SeasonNumber != 2 || item.EpisodeNumber != 5 {
		t.Fatalf("episode metadata = %#v", item)
	}
	if item.PosterURL == "" {
		t.Fatalf("expected poster URL falling back to series image")
	}
}

func TestNewForYouWithoutLibraryIDsSkipsRequest(t *testing.T) {
	items, err := NewClient("http://unused", "device", "key").NewForYou(context.Background(), "user-1", nil)
	if err != nil || items != nil {
		t.Fatalf("NewForYou() = %#v, %v, want nil, nil", items, err)
	}
}

func TestNewForYouMergesMultipleLibrariesAndCapsAtFifteen(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		parentID := request.URL.Query().Get("ParentId")
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte("["))
		for i := 0; i < 10; i++ {
			if i > 0 {
				writer.Write([]byte(","))
			}
			date := time.Now().UTC().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			id := parentID + "-" + string(rune('a'+i))
			writer.Write([]byte(`{"Id":"` + id + `","Name":"Item","DateCreated":"` + date + `"}`))
		}
		writer.Write([]byte("]"))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").NewForYou(context.Background(), "user-1", []string{"3", "5"})
	if err != nil {
		t.Fatalf("NewForYou() error = %v", err)
	}
	if len(items) != 15 {
		t.Fatalf("len(items) = %d, want 15", len(items))
	}
}
