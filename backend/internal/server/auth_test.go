package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/seerr"
)

type fakeAuthenticator struct {
	identity emby.Identity
	err      error
}

func (auth fakeAuthenticator) Authenticate(context.Context, emby.Credentials) (emby.Identity, error) {
	return auth.identity, auth.err
}

type fakeStatisticsReader struct {
	statistics emby.PersonalWatchTime
	userID     string
	period     string
}

func (reader *fakeStatisticsReader) PersonalWatchTime(_ context.Context, userID, period string) (emby.PersonalWatchTime, error) {
	reader.userID = userID
	reader.period = period
	return reader.statistics, nil
}

type fakeUpcomingReader struct {
	items      []emby.UpcomingItem
	libraryIDs []string
}

func (reader *fakeUpcomingReader) Upcoming(_ context.Context, libraryIDs []string) ([]emby.UpcomingItem, error) {
	reader.libraryIDs = libraryIDs
	return reader.items, nil
}

type fakeNewForYouReader struct {
	items      []emby.NewForYouItem
	userID     string
	libraryIDs []string
}

func (reader *fakeNewForYouReader) NewForYou(_ context.Context, userID string, libraryIDs []string) ([]emby.NewForYouItem, error) {
	reader.userID = userID
	reader.libraryIDs = libraryIDs
	return reader.items, nil
}

type fakeContinueWatchingReader struct {
	items  []emby.ContinueWatchingItem
	userID string
}

func (reader *fakeContinueWatchingReader) ContinueWatching(_ context.Context, userID string) ([]emby.ContinueWatchingItem, error) {
	reader.userID = userID
	return reader.items, nil
}

type fakeWatchedReader struct {
	movies           []emby.WatchedItem
	series           []emby.WatchedItem
	moviesUser       string
	seriesUser       string
	moviesLibraryIDs []string
	seriesLibraryIDs []string
}

func (reader *fakeWatchedReader) WatchedMovies(_ context.Context, userID string, libraryIDs []string) ([]emby.WatchedItem, error) {
	reader.moviesUser = userID
	reader.moviesLibraryIDs = libraryIDs
	return reader.movies, nil
}

func (reader *fakeWatchedReader) WatchedSeries(_ context.Context, userID string, libraryIDs []string) ([]emby.WatchedItem, error) {
	reader.seriesUser = userID
	reader.seriesLibraryIDs = libraryIDs
	return reader.series, nil
}

type fakeRequestsReader struct {
	items      []seerr.Request
	embyUserID string
}

func (reader *fakeRequestsReader) Requests(_ context.Context, embyUserID string) ([]seerr.Request, error) {
	reader.embyUserID = embyUserID
	return reader.items, nil
}

type fakeDiscoverReader struct {
	trending      []seerr.DiscoverItem
	popularMovies []seerr.DiscoverItem
}

func (reader *fakeDiscoverReader) Trending(context.Context) ([]seerr.DiscoverItem, error) {
	return reader.trending, nil
}
func (reader *fakeDiscoverReader) PopularMovies(context.Context) ([]seerr.DiscoverItem, error) {
	return reader.popularMovies, nil
}
func (reader *fakeDiscoverReader) PopularSeries(context.Context) ([]seerr.DiscoverItem, error) {
	return nil, nil
}
func (reader *fakeDiscoverReader) UpcomingMovies(context.Context) ([]seerr.DiscoverItem, error) {
	return nil, nil
}
func (reader *fakeDiscoverReader) UpcomingSeries(context.Context) ([]seerr.DiscoverItem, error) {
	return nil, nil
}

type memorySessionStore struct {
	identity emby.Identity
	deleted  bool
}

func (store *memorySessionStore) Create(_ context.Context, identity emby.Identity) (string, error) {
	store.identity = identity
	return "session-id", nil
}

func (store *memorySessionStore) Get(_ context.Context, identifier string) (emby.Identity, error) {
	if identifier != "session-id" {
		return emby.Identity{}, errors.New("unknown session")
	}
	return store.identity, nil
}

func (store *memorySessionStore) Delete(_ context.Context, _ string) error {
	store.deleted = true
	return nil
}

func TestLoginCreatesSessionWithoutExposingEmbyToken(t *testing.T) {
	store := &memorySessionStore{}
	app := &App{
		authenticator: fakeAuthenticator{identity: emby.Identity{UserID: "user-1", DisplayName: "Thomas", AccessToken: "secret-token"}},
		sessions:      store,
		cookieSecure:  true,
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(`{"username":"thomas","password":"password"}`))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-token") {
		t.Fatal("login response exposed Emby access token")
	}
	cookie := recorder.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.Value != "session-id" {
		t.Fatalf("cookie = %#v", cookie)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	app := &App{authenticator: fakeAuthenticator{err: emby.ErrInvalidCredentials}, sessions: &memorySessionStore{}}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/emby/login", strings.NewReader(`{"username":"thomas","password":"wrong"}`))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestStatsUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1", DisplayName: "Thomas", AccessToken: "secret-token"}}
	statistics := &fakeStatisticsReader{statistics: emby.PersonalWatchTime{WatchSeconds: 3600, PreviousWatchSeconds: 1800}}
	app := &App{sessions: store, statistics: statistics}
	request := httptest.NewRequest(http.MethodGet, "/api/stats?period=month", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if statistics.userID != "user-1" || statistics.period != "month" {
		t.Fatalf("statistics request = user %q, period %q", statistics.userID, statistics.period)
	}
	if strings.Contains(recorder.Body.String(), "secret-token") {
		t.Fatal("statistics response exposed Emby access token")
	}
}

func TestUpcomingRequiresAuthentication(t *testing.T) {
	app := &App{sessions: &memorySessionStore{}, upcoming: &fakeUpcomingReader{}}
	request := httptest.NewRequest(http.MethodGet, "/api/upcoming", nil)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUpcomingUsesConfiguredLibraryIDs(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeUpcomingReader{items: []emby.UpcomingItem{{ID: "1", Title: "Alien: Earth"}}}
	app := &App{sessions: store, upcoming: reader, comingSoonLibraryIDs: []string{"library-1"}}
	request := httptest.NewRequest(http.MethodGet, "/api/upcoming", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if len(reader.libraryIDs) != 1 || reader.libraryIDs[0] != "library-1" {
		t.Fatalf("libraryIDs = %#v", reader.libraryIDs)
	}
	if !strings.Contains(recorder.Body.String(), "Alien: Earth") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestNewForYouUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeNewForYouReader{items: []emby.NewForYouItem{{ID: "1", Title: "Sinners"}}}
	app := &App{sessions: store, newForYou: reader}
	request := httptest.NewRequest(http.MethodGet, "/api/new-for-you", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.userID != "user-1" {
		t.Fatalf("userID = %q", reader.userID)
	}
	if !strings.Contains(recorder.Body.String(), "Sinners") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestContinueWatchingUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeContinueWatchingReader{items: []emby.ContinueWatchingItem{{ID: "1", Title: "The Bear"}}}
	app := &App{sessions: store, continueWatching: reader}
	request := httptest.NewRequest(http.MethodGet, "/api/continue-watching", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.userID != "user-1" {
		t.Fatalf("userID = %q", reader.userID)
	}
	if !strings.Contains(recorder.Body.String(), "The Bear") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestWatchedMoviesAndSeriesUseSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeWatchedReader{
		movies: []emby.WatchedItem{{ID: "1", Title: "Dune"}},
		series: []emby.WatchedItem{{ID: "2", Title: "Severance"}},
	}
	app := &App{sessions: store, watched: reader, watchedLibraryIDs: []string{"3", "5", "123857"}}

	moviesRequest := httptest.NewRequest(http.MethodGet, "/api/watched-movies", nil)
	moviesRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	moviesRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(moviesRecorder, moviesRequest)
	if moviesRecorder.Code != http.StatusOK || !strings.Contains(moviesRecorder.Body.String(), "Dune") {
		t.Fatalf("watched-movies status = %d, body = %s", moviesRecorder.Code, moviesRecorder.Body.String())
	}
	if reader.moviesUser != "user-1" {
		t.Fatalf("moviesUser = %q", reader.moviesUser)
	}
	if len(reader.moviesLibraryIDs) != 3 {
		t.Fatalf("moviesLibraryIDs = %#v", reader.moviesLibraryIDs)
	}

	seriesRequest := httptest.NewRequest(http.MethodGet, "/api/watched-series", nil)
	seriesRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	seriesRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(seriesRecorder, seriesRequest)
	if seriesRecorder.Code != http.StatusOK || !strings.Contains(seriesRecorder.Body.String(), "Severance") {
		t.Fatalf("watched-series status = %d, body = %s", seriesRecorder.Code, seriesRecorder.Body.String())
	}
	if reader.seriesUser != "user-1" {
		t.Fatalf("seriesUser = %q", reader.seriesUser)
	}
}

func TestMyRequestsUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeRequestsReader{items: []seerr.Request{{ID: "1", Title: "Severance", Status: "Angefragt"}}}
	app := &App{sessions: store, requests: reader}
	request := httptest.NewRequest(http.MethodGet, "/api/requests", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.embyUserID != "user-1" {
		t.Fatalf("embyUserID = %q", reader.embyUserID)
	}
	if !strings.Contains(recorder.Body.String(), "Severance") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestDiscoverEndpointsRequireAuthAndReturnData(t *testing.T) {
	reader := &fakeDiscoverReader{
		trending:      []seerr.DiscoverItem{{ID: "1", Title: "Dune", MediaType: "movie"}},
		popularMovies: []seerr.DiscoverItem{{ID: "2", Title: "The Odyssey", MediaType: "movie"}},
	}
	app := &App{sessions: &memorySessionStore{}, discover: reader}

	unauth := httptest.NewRequest(http.MethodGet, "/api/discover/trending", nil)
	unauthRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(unauthRecorder, unauth)
	if unauthRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d", unauthRecorder.Code)
	}

	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app.sessions = store

	trending := httptest.NewRequest(http.MethodGet, "/api/discover/trending", nil)
	trending.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	trendingRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(trendingRecorder, trending)
	if trendingRecorder.Code != http.StatusOK || !strings.Contains(trendingRecorder.Body.String(), "Dune") {
		t.Fatalf("trending status = %d, body = %s", trendingRecorder.Code, trendingRecorder.Body.String())
	}

	popular := httptest.NewRequest(http.MethodGet, "/api/discover/movies/popular", nil)
	popular.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	popularRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(popularRecorder, popular)
	if popularRecorder.Code != http.StatusOK || !strings.Contains(popularRecorder.Body.String(), "The Odyssey") {
		t.Fatalf("popular movies status = %d, body = %s", popularRecorder.Code, popularRecorder.Body.String())
	}
}
