package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/emby"
)

// Ratings are decoration: with OMDb switched off (or never configured, which
// is what a bare &App{} models) the detail must still answer, and the two
// rating fields must be absent from the JSON rather than present and empty.
func TestEmbyMediaDetailWithoutOMDbOmitsRatings(t *testing.T) {
	app := &App{
		sessions:        &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
		embyMediaDetail: &fakeEmbyMediaDetailReader{detail: emby.MediaDetail{ID: "1", Title: "Dune", ImdbID: "tt1160419"}},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/media/emby?id=1", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.embyMediaDetailHandler(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	body := recorder.Body.String()
	for _, field := range []string{"imdbRating", "rottenTomatoesRating"} {
		if strings.Contains(body, field) {
			t.Errorf("%s is in the response even though OMDb is not configured: %s", field, body)
		}
	}
}

// The IMDb id is what OMDb is keyed by, and it must never reach the browser
// — same contract as seerr.MediaDetail.
func TestEmbyMediaDetailKeepsTheImdbIDServerSide(t *testing.T) {
	app := &App{
		sessions:        &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
		embyMediaDetail: &fakeEmbyMediaDetailReader{detail: emby.MediaDetail{ID: "1", Title: "Dune", ImdbID: "tt1160419"}},
	}

	request := httptest.NewRequest(http.MethodGet, "/api/media/emby?id=1", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.embyMediaDetailHandler(recorder, request)

	var body map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, leaked := body["imdbId"]; leaked {
		t.Error("the IMDb id was sent to the browser")
	}
}

// omdbRatings runs on every detail request, so a nil live config (or a nil
// client inside it) must not panic.
func TestOmdbRatingsIsInertWithoutAClient(t *testing.T) {
	app := &App{}
	if imdb, rotten := app.omdbRatings(context.Background(), "tt1160419"); imdb != "" || rotten != "" {
		t.Fatalf("ratings = %q/%q, want empty", imdb, rotten)
	}
	if imdb, rotten := app.omdbRatings(context.Background(), ""); imdb != "" || rotten != "" {
		t.Fatalf("ratings = %q/%q for an empty IMDb id, want empty", imdb, rotten)
	}
}
