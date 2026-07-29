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
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[
			{"Id":"1","Name":"Recent","DateCreated":"` + recent + `","ImageTags":{"Primary":"tag-1"}},
			{"Id":"2","Name":"TooOld","DateCreated":"` + tooOld + `"},
			{"Id":"3","Name":"NoDate"}
		]`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").NewForYou(context.Background(), "user-1")
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

func TestNewForYouCapsAtFifteenItems(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Write([]byte("["))
		for i := 0; i < 20; i++ {
			if i > 0 {
				writer.Write([]byte(","))
			}
			date := time.Now().UTC().Add(-time.Duration(i) * time.Hour).Format(time.RFC3339)
			writer.Write([]byte(`{"Id":"` + string(rune('a'+i)) + `","Name":"Item","DateCreated":"` + date + `"}`))
		}
		writer.Write([]byte("]"))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").NewForYou(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("NewForYou() error = %v", err)
	}
	if len(items) != 15 {
		t.Fatalf("len(items) = %d, want 15", len(items))
	}
}
