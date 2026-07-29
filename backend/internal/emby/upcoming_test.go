package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUpcomingFiltersToNextFourWeeks(t *testing.T) {
	soon := time.Now().UTC().Add(7 * 24 * time.Hour).Format(time.RFC3339)
	tooFar := time.Now().UTC().Add(60 * 24 * time.Hour).Format(time.RFC3339)
	past := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)

	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Items" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		if request.URL.Query().Get("ParentId") != "library-1" {
			t.Fatalf("ParentId = %q", request.URL.Query().Get("ParentId"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Items":[
			{"Id":"1","Name":"Soon","PremiereDate":"` + soon + `","ImageTags":{"Primary":"tag-1"}},
			{"Id":"2","Name":"TooFar","PremiereDate":"` + tooFar + `"},
			{"Id":"3","Name":"Past","PremiereDate":"` + past + `"},
			{"Id":"4","Name":"NoDate"}
		]}`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").Upcoming(context.Background(), []string{"library-1"})
	if err != nil {
		t.Fatalf("Upcoming() error = %v", err)
	}
	if len(items) != 1 || items[0].Title != "Soon" {
		t.Fatalf("items = %#v", items)
	}
	if items[0].PosterURL == "" {
		t.Fatalf("expected poster URL for item with an image tag")
	}
}

func TestUpcomingWithoutLibraryIDsSkipsRequest(t *testing.T) {
	items, err := NewClient("http://unused", "device", "key").Upcoming(context.Background(), nil)
	if err != nil || items != nil {
		t.Fatalf("Upcoming() = %#v, %v, want nil, nil", items, err)
	}
}
