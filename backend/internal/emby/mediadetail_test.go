package emby

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEmbyMediaDetailSplitsCastAndCrew(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/emby/Users/user-1/Items/154950" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		if request.Header.Get("X-Emby-Token") != "admin-key" {
			t.Fatalf("api key header = %q", request.Header.Get("X-Emby-Token"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"Id":"154950","Name":"Dune","Type":"Movie","Overview":"Sci-fi epic",
			"Genres":["Science Fiction"],"CommunityRating":7.8,"OfficialRating":"12",
			"ProductionYear":2026,"RunTimeTicks":79511420000,
			"ImageTags":{"Primary":"poster-tag"},"BackdropImageTags":["backdrop-tag"],
			"People":[
				{"Id":"1","Name":"Denis Villeneuve","Type":"Director","PrimaryImageTag":"tag-1"},
				{"Id":"2","Name":"Timothée Chalamet","Role":"Paul Atreides","Type":"Actor","PrimaryImageTag":"tag-2"}
			],
			"UserData":{"Played":true,"UnplayedItemCount":0}
		}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").EmbyMediaDetail(context.Background(), "user-1", "154950")
	if err != nil {
		t.Fatalf("EmbyMediaDetail() error = %v", err)
	}
	if detail.Title != "Dune" || detail.Overview != "Sci-fi epic" || !detail.Played {
		t.Fatalf("detail = %#v", detail)
	}
	if detail.RuntimeMinutes != 132 {
		t.Fatalf("RuntimeMinutes = %d, want 132", detail.RuntimeMinutes)
	}
	if detail.PosterURL == "" || detail.BackdropURL == "" {
		t.Fatalf("expected poster and backdrop URLs, got %#v", detail)
	}
	if len(detail.Cast) != 1 || detail.Cast[0].Name != "Timothée Chalamet" || detail.Cast[0].Role != "Paul Atreides" {
		t.Fatalf("cast = %#v", detail.Cast)
	}
	if len(detail.Crew) != 1 || detail.Crew[0].Name != "Denis Villeneuve" || detail.Crew[0].Role != "Director" {
		t.Fatalf("crew = %#v", detail.Crew)
	}
}

func TestEmbyMediaDetailComputesSeriesProgress(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{
			"Id":"1","Name":"Severance","Type":"Series",
			"RecursiveItemCount":18,
			"UserData":{"Played":false,"UnplayedItemCount":3}
		}`))
	}))
	defer testServer.Close()

	detail, err := NewClient(testServer.URL+"/emby", "dashboard-device", "admin-key").EmbyMediaDetail(context.Background(), "user-1", "1")
	if err != nil {
		t.Fatalf("EmbyMediaDetail() error = %v", err)
	}
	if !detail.IsSeries || detail.TotalEpisodes != 18 || detail.WatchedEpisodes != 15 {
		t.Fatalf("detail = %#v", detail)
	}
}
