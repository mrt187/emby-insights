package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/tracearr"
)

// fakeTracearr serves the handful of Tracearr v2 responses the handlers
// need, so the server tests exercise the real client end to end.
func fakeTracearr(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/api/v2/public/users":
			_, _ = writer.Write([]byte(`{"data":[
				{"id":"identity-1","accounts":[{"server_type":"emby","external_user_id":"user-1"}]}
			],"meta":{"nextCursor":null,"pageSize":100}}`))
		case request.URL.Path == "/api/v2/public/users/identity-1/stats":
			_, _ = writer.Write([]byte(`{"user_id":"identity-1","top_genres":[{"genre":"Sci-Fi","plays":31}]}`))
		case request.URL.Path == "/api/v2/public/history":
			if request.URL.Query().Get("watched") == "false" {
				_, _ = writer.Write([]byte(`{"data":[
					{"media_id":"m1","media_type":"movie","media_title":"Abandoned Film","percent_complete":40}
				],"meta":{"nextCursor":null,"pageSize":100}}`))
				return
			}
			_, _ = writer.Write([]byte(`{"data":[
				{"media_id":"m1","is_transcode":true},{"media_id":"m2","is_transcode":false}
			],"meta":{"nextCursor":null,"pageSize":100}}`))
		case strings.HasSuffix(request.URL.Path, "/stats"):
			_, _ = writer.Write([]byte(`{"windows":{"all_time":{"combined":{"plays":9,"watch_time_ms":100,"unique_users":3},"per_server":[]}}}`))
		case strings.HasSuffix(request.URL.Path, "/watchers"):
			_, _ = writer.Write([]byte(`{"media_id":"m1","media_type":"movie","window":"all_time","watchers":[
				{"user":{"username":"alex","identity_name":"Alex"},"plays":2,"completion_pct":99}
			]}`))
		default:
			t.Errorf("unexpected Tracearr path %q", request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
}

func tracearrLive(t *testing.T) (*liveConfig, *httptest.Server) {
	t.Helper()
	upstream := fakeTracearr(t)
	live := &liveConfig{}
	live.set(nil, nil, nil, tracearr.NewClient(upstream.URL, "test-key"), "", nil, nil)
	return live, upstream
}

func getAs(t *testing.T, app *App, target string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, target, nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)
	return recorder
}

func TestTracearrStatsEndpoints(t *testing.T) {
	live, upstream := tracearrLive(t)
	defer upstream.Close()
	app := &App{
		sessions: &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
		live:     live,
	}

	for _, testCase := range []struct{ path, want string }{
		{"/api/stats/genres", "Sci-Fi"},
		{"/api/stats/unfinished?period=month", "Abandoned Film"},
		{"/api/stats/transcode-share", `"transcodes":1`},
	} {
		recorder := getAs(t, app, testCase.path)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", testCase.path, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), testCase.want) {
			t.Fatalf("%s: body = %s, want %s", testCase.path, recorder.Body.String(), testCase.want)
		}
	}
}

// Disabling Tracearr — or pointing it at a dead host — must leave every
// endpoint answering normally with nothing in it, never erroring. This is
// the property the whole integration rests on: it is decoration.
func TestTracearrEndpointsStayHealthyWhenUnavailable(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	disabled := &liveConfig{}
	disabled.set(nil, nil, nil, nil, "", nil, nil)
	broken := &liveConfig{}
	broken.set(nil, nil, nil, tracearr.NewClient(dead.URL, "test-key"), "", nil, nil)

	for name, live := range map[string]*liveConfig{"disabled": disabled, "unreachable": broken} {
		app := &App{
			sessions: &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
			live:     live,
		}
		for path, want := range map[string]string{
			"/api/stats/genres":          "[]",
			"/api/stats/unfinished":      "[]",
			"/api/stats/transcode-share": `{"plays":0,"transcodes":0}`,
		} {
			recorder := getAs(t, app, path)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s %s: status = %d", name, path, recorder.Code)
			}
			if strings.TrimSpace(recorder.Body.String()) != want {
				t.Fatalf("%s %s: body = %s, want %s", name, path, recorder.Body.String(), want)
			}
		}
	}
}

func TestEmbyMediaDetailCarriesHouseholdOnlyWhenTracearrIsOn(t *testing.T) {
	detail := emby.MediaDetail{ID: "154950", Title: "Dune", TmdbID: "438631"}

	live, upstream := tracearrLive(t)
	defer upstream.Close()
	withTracearr := &App{
		sessions:        &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
		embyMediaDetail: &fakeEmbyMediaDetailReader{detail: detail},
		live:            live,
	}
	recorder := getAs(t, withTracearr, "/api/media/emby?id=154950")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Title     string `json:"title"`
		Household *struct {
			Plays       int `json:"plays"`
			UniqueUsers int `json:"uniqueUsers"`
			Watchers    []struct {
				Name string `json:"name"`
			} `json:"watchers"`
		} `json:"household"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v, body = %s", err, recorder.Body.String())
	}
	if response.Title != "Dune" {
		t.Fatalf("title = %q — the detail payload must survive the wrapping", response.Title)
	}
	if response.Household == nil || response.Household.Plays != 9 || len(response.Household.Watchers) != 1 {
		t.Fatalf("household = %#v", response.Household)
	}
	if response.Household.Watchers[0].Name != "Alex" {
		t.Fatalf("watcher = %q", response.Household.Watchers[0].Name)
	}

	// Emby's external ids stay server-side: they exist to address Tracearr,
	// and the browser never had them before this integration either.
	if strings.Contains(recorder.Body.String(), "438631") {
		t.Fatalf("media detail leaked provider ids: %s", recorder.Body.String())
	}

	withoutTracearr := &App{
		sessions:        &memorySessionStore{identity: emby.Identity{UserID: "user-1"}},
		embyMediaDetail: &fakeEmbyMediaDetailReader{detail: detail},
		live:            &liveConfig{},
	}
	recorder = getAs(t, withoutTracearr, "/api/media/emby?id=154950")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "household") {
		t.Fatalf("household must be omitted when Tracearr is off: %s", recorder.Body.String())
	}
}

func TestTracearrStatsRequireASession(t *testing.T) {
	live, upstream := tracearrLive(t)
	defer upstream.Close()
	app := &App{sessions: &memorySessionStore{}, live: live}

	request := httptest.NewRequest(http.MethodGet, "/api/stats/genres", nil)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestPeriodStartWindows(t *testing.T) {
	week, month, year := periodStart("week"), periodStart("month"), periodStart("year")
	if !week.After(month) || !month.After(year) {
		t.Fatalf("windows are not ordered: week %v, month %v, year %v", week, month, year)
	}
	// An unrecognised period must fall back to the shortest window rather
	// than silently reading a year of history.
	if periodStart("nonsense").Before(year) {
		t.Fatal("unknown period reached further back than a year")
	}
}
