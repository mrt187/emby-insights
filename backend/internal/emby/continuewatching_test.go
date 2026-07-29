package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContinueWatchingBuildsTitleAndProgress(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1/Items/Resume" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"Items":[
			{"Id":"1","Name":"The Bear","RunTimeTicks":36000000000,"UserData":{"PlaybackPositionTicks":18000000000}},
			{"Id":"2","Name":"Chapter 4","SeriesName":"Severance","SeriesId":"10","SeriesPrimaryImageTag":"tag-series","RunTimeTicks":30000000000,"UserData":{"PlaybackPositionTicks":15000000000}}
		]}`))
	}))
	defer testServer.Close()

	items, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").ContinueWatching(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ContinueWatching() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %#v", items)
	}
	if items[0].Title != "The Bear" || items[0].ProgressPercent != 50 {
		t.Fatalf("items[0] = %#v", items[0])
	}
	if items[1].Title != "Severance – Chapter 4" || items[1].ProgressPercent != 50 {
		t.Fatalf("items[1] = %#v", items[1])
	}
	if items[1].PosterURL == "" {
		t.Fatalf("expected poster URL sourced from the series image")
	}
}
