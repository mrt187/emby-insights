package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/comingsoon"
	"github.com/mrt187/EmbyInsights/internal/emby"
)

func comingSoonDetailApp(items ...comingsoon.Item) (*App, *http.Cookie) {
	return &App{
			sessions:   &memorySessionStore{identity: emby.Identity{UserID: "user-1", DisplayName: "Tom"}},
			comingSoon: &fakeComingSoonReader{upcoming: items},
		},
		&http.Cookie{Name: sessionCookieName, Value: "session-id"}
}

func getComingSoonDetail(app *App, cookie *http.Cookie, query string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/api/media/comingsoon?"+query, nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	app.comingSoonMediaDetailHandler(recorder, request)
	return recorder
}

// TestComingSoonDetailWorksWithoutSeerr is the point of the endpoint: a Sonarr
// entry has no TMDB id when TMDB is not configured, so it cannot be looked up
// through Seerr at all — but Sonarr already told us everything this screen
// shows.
func TestComingSoonDetailWorksWithoutSeerr(t *testing.T) {
	app, cookie := comingSoonDetailApp(comingsoon.Item{
		ID: "42", Source: comingsoon.SourceSonarr, DetailID: "42", TmdbID: "",
		Title: "Eine Serie", MediaType: "tv", PosterURL: "/api/artwork?u=x",
		Overview: "Beschreibung.", Genres: []string{"Krimi"}, RuntimeMinutes: 45,
		Year: 2024, OfficialRating: "FSK 16", Rating: 8.1, Studio: "ARD",
	})

	recorder := getComingSoonDetail(app, cookie, "source=sonarr&id=42")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	var detail struct {
		Title          string   `json:"title"`
		Overview       string   `json:"overview"`
		Genres         []string `json:"genres"`
		RuntimeMinutes int      `json:"runtimeMinutes"`
		Year           int      `json:"year"`
		OfficialRating string   `json:"officialRating"`
		Studios        []string `json:"studios"`
		Cast           []any    `json:"cast"`
		Seasons        []any    `json:"seasons"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.Title != "Eine Serie" || detail.Overview != "Beschreibung." {
		t.Errorf("detail = %+v", detail)
	}
	if detail.RuntimeMinutes != 45 || detail.Year != 2024 || detail.OfficialRating != "FSK 16" {
		t.Errorf("runtime/year/rating = %d/%d/%q", detail.RuntimeMinutes, detail.Year, detail.OfficialRating)
	}
	if len(detail.Studios) != 1 || detail.Studios[0] != "ARD" {
		t.Errorf("studios = %v", detail.Studios)
	}
	// Radarr and Sonarr carry neither, and the frontend maps over both.
	if detail.Cast == nil || detail.Seasons == nil {
		t.Error("cast and seasons must be empty arrays, not null")
	}
}

func TestComingSoonDetailRejectsBadInput(t *testing.T) {
	app, cookie := comingSoonDetailApp(comingsoon.Item{ID: "7", Source: comingsoon.SourceRadarr, DetailID: "7", Title: "Film"})

	for _, query := range []string{"", "source=radarr", "id=7", "source=tmdb&id=7", "source=&id=7", "source=radarr&id="} {
		if code := getComingSoonDetail(app, cookie, query).Code; code != http.StatusBadRequest {
			t.Errorf("query %q answered %d, want 400", query, code)
		}
	}
	if code := getComingSoonDetail(app, cookie, "source=sonarr&id=7").Code; code != http.StatusNotFound {
		t.Errorf("a Radarr id under sonarr answered %d, want 404", code)
	}
	if code := getComingSoonDetail(app, cookie, "source=radarr&id=999").Code; code != http.StatusNotFound {
		t.Errorf("unknown id answered %d, want 404", code)
	}
}

func TestComingSoonDetailRequiresAuthentication(t *testing.T) {
	app, _ := comingSoonDetailApp()
	recorder := httptest.NewRecorder()
	app.comingSoonMediaDetailHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/media/comingsoon?source=radarr&id=7", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}
