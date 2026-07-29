package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/seerr"
	"github.com/mrt187/EmbyInsights/internal/store"
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

type fakeProfileReader struct {
	profile emby.UserProfile
	userID  string
}

func (reader *fakeProfileReader) UserProfile(_ context.Context, userID string) (emby.UserProfile, error) {
	reader.userID = userID
	return reader.profile, nil
}

type fakeRequestStatsReader struct {
	stats  seerr.RequestStats
	userID string
}

func (reader *fakeRequestStatsReader) RequestStats(_ context.Context, userID string) (seerr.RequestStats, error) {
	reader.userID = userID
	return reader.stats, nil
}

type fakeDeviceStatisticsReader struct {
	devices []emby.DeviceWatchTime
	userID  string
	period  string
}

func (reader *fakeDeviceStatisticsReader) DeviceWatchTimes(_ context.Context, userID, period string) ([]emby.DeviceWatchTime, error) {
	reader.userID = userID
	reader.period = period
	return reader.devices, nil
}

type fakeSessionStatisticsReader struct {
	hours            []emby.HourWatchTime
	weekdays         []emby.WeekdayWatchTime
	longestSession   emby.LongestSession
	longestSessionOK bool
	mostActiveDay    emby.MostActiveDay
	mostActiveDayOK  bool
	err              error
}

func (reader *fakeSessionStatisticsReader) HourWatchTimes(context.Context, string, string) ([]emby.HourWatchTime, error) {
	return reader.hours, reader.err
}
func (reader *fakeSessionStatisticsReader) WeekdayWatchTimes(context.Context, string, string) ([]emby.WeekdayWatchTime, error) {
	return reader.weekdays, reader.err
}
func (reader *fakeSessionStatisticsReader) LongestSession(context.Context, string, string) (emby.LongestSession, bool, error) {
	return reader.longestSession, reader.longestSessionOK, reader.err
}
func (reader *fakeSessionStatisticsReader) MostActiveDay(context.Context, string, string) (emby.MostActiveDay, bool, error) {
	return reader.mostActiveDay, reader.mostActiveDayOK, reader.err
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

type fakeCompletedReader struct {
	movies     []emby.WatchedItem
	series     []emby.WatchedItem
	moviesUser string
	seriesUser string
	moviesFrom time.Time
	moviesTo   time.Time
}

func (reader *fakeCompletedReader) CompletedMovies(_ context.Context, userID string, _ []string, from, to time.Time) ([]emby.WatchedItem, error) {
	reader.moviesUser = userID
	reader.moviesFrom = from
	reader.moviesTo = to
	return reader.movies, nil
}

func (reader *fakeCompletedReader) CompletedSeries(_ context.Context, userID string, _ []string, _, _ time.Time) ([]emby.WatchedItem, error) {
	reader.seriesUser = userID
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

type fakeAvailableRequestsReader struct {
	items      []seerr.Request
	embyUserID string
	since      time.Time
}

func (reader *fakeAvailableRequestsReader) AvailableRequests(_ context.Context, embyUserID string, since time.Time) ([]seerr.Request, error) {
	reader.embyUserID = embyUserID
	reader.since = since
	return reader.items, nil
}

type fakeDiscoverReader struct {
	trending      []seerr.DiscoverItem
	popularMovies []seerr.DiscoverItem
	searchResults []seerr.DiscoverItem
	searchQuery   string
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
func (reader *fakeDiscoverReader) Search(_ context.Context, query string) ([]seerr.DiscoverItem, error) {
	reader.searchQuery = query
	return reader.searchResults, nil
}

type fakeEmbyMediaDetailReader struct {
	detail emby.MediaDetail
	userID string
	itemID string
}

func (reader *fakeEmbyMediaDetailReader) EmbyMediaDetail(_ context.Context, userID, itemID string) (emby.MediaDetail, error) {
	reader.userID = userID
	reader.itemID = itemID
	return reader.detail, nil
}

type fakeSeerrMediaDetailReader struct {
	detail    seerr.MediaDetail
	mediaType string
	tmdbID    int
}

func (reader *fakeSeerrMediaDetailReader) MediaDetail(_ context.Context, mediaType string, tmdbID int) (seerr.MediaDetail, error) {
	reader.mediaType = mediaType
	reader.tmdbID = tmdbID
	return reader.detail, nil
}

type fakeRequestCreator struct {
	err        error
	embyUserID string
	mediaType  string
	tmdbID     int
	seasons    []int
}

func (creator *fakeRequestCreator) CreateRequest(_ context.Context, embyUserID, mediaType string, tmdbID int, seasons []int) error {
	creator.embyUserID = embyUserID
	creator.mediaType = mediaType
	creator.tmdbID = tmdbID
	creator.seasons = seasons
	return creator.err
}

type fakeTrackingStore struct {
	getEntry     store.MediaTracking
	getFound     bool
	getErr       error
	upsertUserID string
	upserted     store.MediaTracking
	upsertErr    error
	watchlist    []store.MediaTracking
	ratings      []store.MediaTracking
	err          error
}

func (fake *fakeTrackingStore) Get(_ context.Context, _, _, _ string) (store.MediaTracking, bool, error) {
	return fake.getEntry, fake.getFound, fake.getErr
}

func (fake *fakeTrackingStore) Upsert(_ context.Context, embyUserID string, entry store.MediaTracking) error {
	fake.upsertUserID = embyUserID
	fake.upserted = entry
	return fake.upsertErr
}

func (fake *fakeTrackingStore) Watchlist(_ context.Context, _ string) ([]store.MediaTracking, error) {
	return fake.watchlist, fake.err
}

func (fake *fakeTrackingStore) Ratings(_ context.Context, _ string) ([]store.MediaTracking, error) {
	return fake.ratings, fake.err
}

type fakeFavoriteWriter struct {
	userID   string
	itemID   string
	favorite bool
	err      error
}

func (fake *fakeFavoriteWriter) SetFavorite(_ context.Context, userID, itemID string, favorite bool) error {
	fake.userID = userID
	fake.itemID = itemID
	fake.favorite = favorite
	return fake.err
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

func TestDeviceStatsUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeDeviceStatisticsReader{devices: []emby.DeviceWatchTime{{DeviceName: "FireTV 4K", WatchSeconds: 3600}}}
	app := &App{sessions: store, deviceStatistics: reader}

	request := httptest.NewRequest(http.MethodGet, "/api/stats/devices?period=year", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "FireTV 4K") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.userID != "user-1" || reader.period != "year" {
		t.Fatalf("userID = %q, period = %q", reader.userID, reader.period)
	}
}

func TestHourAndWeekdayStatsRequireAuthAndReturnData(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeSessionStatisticsReader{
		hours:    []emby.HourWatchTime{{Hour: 21, WatchSeconds: 7200}},
		weekdays: []emby.WeekdayWatchTime{{Weekday: 0, WatchSeconds: 1800}},
	}
	app := &App{sessions: store, sessionStatistics: reader}

	hoursRequest := httptest.NewRequest(http.MethodGet, "/api/stats/hours", nil)
	hoursRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	hoursRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(hoursRecorder, hoursRequest)
	if hoursRecorder.Code != http.StatusOK || !strings.Contains(hoursRecorder.Body.String(), "7200") {
		t.Fatalf("hours status = %d, body = %s", hoursRecorder.Code, hoursRecorder.Body.String())
	}

	weekdaysRequest := httptest.NewRequest(http.MethodGet, "/api/stats/weekdays", nil)
	weekdaysRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	weekdaysRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(weekdaysRecorder, weekdaysRequest)
	if weekdaysRecorder.Code != http.StatusOK || !strings.Contains(weekdaysRecorder.Body.String(), "1800") {
		t.Fatalf("weekdays status = %d, body = %s", weekdaysRecorder.Code, weekdaysRecorder.Body.String())
	}

	unauth := httptest.NewRequest(http.MethodGet, "/api/stats/hours", nil)
	unauthRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(unauthRecorder, unauth)
	if unauthRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d", unauthRecorder.Code)
	}
}

func TestLongestSessionAndMostActiveDayHandleNotFound(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: store, sessionStatistics: &fakeSessionStatisticsReader{}}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/stats/longest-session", nil)
	sessionRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	sessionRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK || strings.TrimSpace(sessionRecorder.Body.String()) != "null" {
		t.Fatalf("longest-session status = %d, body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	dayRequest := httptest.NewRequest(http.MethodGet, "/api/stats/most-active-day", nil)
	dayRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	dayRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(dayRecorder, dayRequest)
	if dayRecorder.Code != http.StatusOK || strings.TrimSpace(dayRecorder.Body.String()) != "null" {
		t.Fatalf("most-active-day status = %d, body = %s", dayRecorder.Code, dayRecorder.Body.String())
	}
}

func TestLongestSessionAndMostActiveDayReturnData(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeSessionStatisticsReader{
		longestSession:   emby.LongestSession{ItemName: "Shelter", WatchSeconds: 12531},
		longestSessionOK: true,
		mostActiveDay:    emby.MostActiveDay{Date: "2026-02-01", WatchSeconds: 31924},
		mostActiveDayOK:  true,
	}
	app := &App{sessions: store, sessionStatistics: reader}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/stats/longest-session", nil)
	sessionRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	sessionRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(sessionRecorder, sessionRequest)
	if sessionRecorder.Code != http.StatusOK || !strings.Contains(sessionRecorder.Body.String(), "Shelter") {
		t.Fatalf("longest-session status = %d, body = %s", sessionRecorder.Code, sessionRecorder.Body.String())
	}

	dayRequest := httptest.NewRequest(http.MethodGet, "/api/stats/most-active-day", nil)
	dayRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	dayRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(dayRecorder, dayRequest)
	if dayRecorder.Code != http.StatusOK || !strings.Contains(dayRecorder.Body.String(), "31924") {
		t.Fatalf("most-active-day status = %d, body = %s", dayRecorder.Code, dayRecorder.Body.String())
	}
}

func TestMeProfileUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	profileReader := &fakeProfileReader{profile: emby.UserProfile{MemberSince: "2026-01-14T08:00:48Z", LastActiveDate: "2026-07-29T13:59:59Z"}}
	app := &App{sessions: store, profile: profileReader}

	request := httptest.NewRequest(http.MethodGet, "/api/me/profile", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "2026-01-14T08:00:48Z") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if profileReader.userID != "user-1" {
		t.Fatalf("userID = %q", profileReader.userID)
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

func TestCompletedMoviesAndSeriesUsePeriodAndSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeCompletedReader{
		movies: []emby.WatchedItem{{ID: "1", Title: "Dune"}},
		series: []emby.WatchedItem{{ID: "2", Title: "Severance"}},
	}
	app := &App{sessions: store, completed: reader, watchedLibraryIDs: []string{"3", "5"}}

	moviesRequest := httptest.NewRequest(http.MethodGet, "/api/completed-movies?period=month", nil)
	moviesRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	moviesRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(moviesRecorder, moviesRequest)
	if moviesRecorder.Code != http.StatusOK || !strings.Contains(moviesRecorder.Body.String(), "Dune") {
		t.Fatalf("completed-movies status = %d, body = %s", moviesRecorder.Code, moviesRecorder.Body.String())
	}
	if reader.moviesUser != "user-1" {
		t.Fatalf("moviesUser = %q", reader.moviesUser)
	}
	if reader.moviesFrom.IsZero() || reader.moviesTo.IsZero() || !reader.moviesFrom.Before(reader.moviesTo) {
		t.Fatalf("moviesFrom/moviesTo = %v/%v, want a resolved period", reader.moviesFrom, reader.moviesTo)
	}

	seriesRequest := httptest.NewRequest(http.MethodGet, "/api/completed-series", nil)
	seriesRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	seriesRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(seriesRecorder, seriesRequest)
	if seriesRecorder.Code != http.StatusOK || !strings.Contains(seriesRecorder.Body.String(), "Severance") {
		t.Fatalf("completed-series status = %d, body = %s", seriesRecorder.Code, seriesRecorder.Body.String())
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

func TestAvailableRequestsUsesSessionIdentityAndWindow(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeAvailableRequestsReader{items: []seerr.Request{{ID: "1", Title: "Alien: Romulus", Status: "Jetzt verfügbar"}}}
	app := &App{sessions: store, availableRequests: reader}

	request := httptest.NewRequest(http.MethodGet, "/api/requests/available", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Alien: Romulus") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.embyUserID != "user-1" {
		t.Fatalf("embyUserID = %q", reader.embyUserID)
	}
	if elapsed := time.Since(reader.since); elapsed < availableRequestWindow || elapsed > availableRequestWindow+time.Minute {
		t.Fatalf("since = %v, want roughly %v ago", reader.since, availableRequestWindow)
	}
}

func TestAvailableRequestsRequiresSession(t *testing.T) {
	app := &App{sessions: &memorySessionStore{}, availableRequests: &fakeAvailableRequestsReader{}}
	request := httptest.NewRequest(http.MethodGet, "/api/requests/available", nil)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestRequestsTotalUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeRequestStatsReader{stats: seerr.RequestStats{Total: 302}}
	app := &App{sessions: store, requestStats: reader}

	request := httptest.NewRequest(http.MethodGet, "/api/requests/total", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "302") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.userID != "user-1" {
		t.Fatalf("userID = %q", reader.userID)
	}
}

func TestDiscoverSearchRequiresQuery(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeDiscoverReader{searchResults: []seerr.DiscoverItem{{ID: "13", Title: "Forrest Gump", MediaType: "movie"}}}
	app := &App{sessions: store, discover: reader}

	missing := httptest.NewRequest(http.MethodGet, "/api/discover/search", nil)
	missing.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	missingRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(missingRecorder, missing)
	if missingRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", missingRecorder.Code, missingRecorder.Body.String())
	}

	good := httptest.NewRequest(http.MethodGet, "/api/discover/search?query=forrest", nil)
	good.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	goodRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(goodRecorder, good)
	if goodRecorder.Code != http.StatusOK || !strings.Contains(goodRecorder.Body.String(), "Forrest Gump") {
		t.Fatalf("status = %d, body = %s", goodRecorder.Code, goodRecorder.Body.String())
	}
	if reader.searchQuery != "forrest" {
		t.Fatalf("searchQuery = %q", reader.searchQuery)
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

func TestEmbyMediaDetailUsesSessionIdentity(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeEmbyMediaDetailReader{detail: emby.MediaDetail{ID: "154950", Title: "Dune"}}
	app := &App{sessions: store, embyMediaDetail: reader}

	request := httptest.NewRequest(http.MethodGet, "/api/media/emby?id=154950", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Dune") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.userID != "user-1" || reader.itemID != "154950" {
		t.Fatalf("userID = %q, itemID = %q", reader.userID, reader.itemID)
	}
}

func TestEmbyMediaDetailRequiresID(t *testing.T) {
	app := &App{sessions: &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}, embyMediaDetail: &fakeEmbyMediaDetailReader{}}
	request := httptest.NewRequest(http.MethodGet, "/api/media/emby", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSeerrMediaDetailValidatesMediaType(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	reader := &fakeSeerrMediaDetailReader{detail: seerr.MediaDetail{ID: "1228710", Title: "The Mandalorian and Grogu"}}
	app := &App{sessions: store, seerrMediaDetail: reader}

	good := httptest.NewRequest(http.MethodGet, "/api/media/seerr?mediaType=movie&id=1228710", nil)
	good.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	goodRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(goodRecorder, good)
	if goodRecorder.Code != http.StatusOK || !strings.Contains(goodRecorder.Body.String(), "Mandalorian") {
		t.Fatalf("status = %d, body = %s", goodRecorder.Code, goodRecorder.Body.String())
	}
	if reader.mediaType != "movie" || reader.tmdbID != 1228710 {
		t.Fatalf("mediaType = %q, tmdbID = %d", reader.mediaType, reader.tmdbID)
	}

	bad := httptest.NewRequest(http.MethodGet, "/api/media/seerr?mediaType=book&id=1228710", nil)
	bad.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	badRecorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(badRecorder, bad)
	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", badRecorder.Code, badRecorder.Body.String())
	}
}

func TestCreateSeerrRequestHandlerSendsSeasons(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	creator := &fakeRequestCreator{}
	app := &App{sessions: store, seerrRequestCreator: creator}

	request := httptest.NewRequest(http.MethodPost, "/api/media/seerr/request", bytes.NewReader([]byte(`{"mediaType":"tv","tmdbId":12345,"seasons":[1,2]}`)))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if creator.embyUserID != "user-1" || creator.mediaType != "tv" || creator.tmdbID != 12345 {
		t.Fatalf("embyUserID = %q, mediaType = %q, tmdbID = %d", creator.embyUserID, creator.mediaType, creator.tmdbID)
	}
	if len(creator.seasons) != 2 || creator.seasons[0] != 1 || creator.seasons[1] != 2 {
		t.Fatalf("seasons = %v", creator.seasons)
	}
}

func TestCreateSeerrRequestHandlerValidatesMediaType(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: store, seerrRequestCreator: &fakeRequestCreator{}}

	request := httptest.NewRequest(http.MethodPost, "/api/media/seerr/request", bytes.NewReader([]byte(`{"mediaType":"book","tmdbId":1}`)))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateSeerrRequestHandlerRequiresTmdbID(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: store, seerrRequestCreator: &fakeRequestCreator{}}

	request := httptest.NewRequest(http.MethodPost, "/api/media/seerr/request", bytes.NewReader([]byte(`{"mediaType":"movie","tmdbId":0}`)))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateSeerrRequestHandlerReturnsBadGatewayOnFailure(t *testing.T) {
	store := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: store, seerrRequestCreator: &fakeRequestCreator{err: errors.New("boom")}}

	request := httptest.NewRequest(http.MethodPost, "/api/media/seerr/request", bytes.NewReader([]byte(`{"mediaType":"movie","tmdbId":1}`)))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateSeerrRequestHandlerRequiresSession(t *testing.T) {
	app := &App{sessions: &memorySessionStore{}, seerrRequestCreator: &fakeRequestCreator{}}

	request := httptest.NewRequest(http.MethodPost, "/api/media/seerr/request", bytes.NewReader([]byte(`{"mediaType":"movie","tmdbId":1}`)))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetTrackingReturnsExistingEntry(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	tracking := &fakeTrackingStore{getEntry: store.MediaTracking{MediaID: "42", Title: "Alien", Rating: 4}, getFound: true}
	app := &App{sessions: sessions, tracking: tracking}

	request := httptest.NewRequest(http.MethodGet, "/api/tracking?source=emby&id=42", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Alien") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestGetTrackingReturnsEmptyEntryWhenUntracked(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: sessions, tracking: &fakeTrackingStore{}}

	request := httptest.NewRequest(http.MethodGet, "/api/tracking?source=emby&id=42", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), `"rating"`) {
		t.Fatalf("expected no rating field for an untracked item, body = %s", recorder.Body.String())
	}
}

func TestGetTrackingRequiresParams(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: sessions, tracking: &fakeTrackingStore{}}

	request := httptest.NewRequest(http.MethodGet, "/api/tracking", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUpsertTrackingSavesEntry(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	tracking := &fakeTrackingStore{}
	app := &App{sessions: sessions, tracking: tracking}

	request := httptest.NewRequest(http.MethodPut, "/api/tracking", bytes.NewReader([]byte(`{"mediaSource":"emby","mediaId":"42","mediaType":"Movie","title":"Alien","rating":5,"onWatchlist":true}`)))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if tracking.upsertUserID != "user-1" || tracking.upserted.MediaID != "42" || tracking.upserted.Rating != 5 || !tracking.upserted.OnWatchlist {
		t.Fatalf("upserted = %#v (userID %q)", tracking.upserted, tracking.upsertUserID)
	}
}

func TestUpsertTrackingValidatesRating(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: sessions, tracking: &fakeTrackingStore{}}

	request := httptest.NewRequest(http.MethodPut, "/api/tracking", bytes.NewReader([]byte(`{"mediaSource":"emby","mediaId":"42","rating":9}`)))
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUpsertTrackingRequiresSession(t *testing.T) {
	app := &App{sessions: &memorySessionStore{}, tracking: &fakeTrackingStore{}}

	request := httptest.NewRequest(http.MethodPut, "/api/tracking", bytes.NewReader([]byte(`{"mediaSource":"emby","mediaId":"42"}`)))
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTrackingWatchlistReturnsEntries(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	tracking := &fakeTrackingStore{watchlist: []store.MediaTracking{{MediaID: "42", Title: "Alien"}}}
	app := &App{sessions: sessions, tracking: tracking}

	request := httptest.NewRequest(http.MethodGet, "/api/tracking/watchlist", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Alien") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTrackingRatingsReturnsEntries(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	tracking := &fakeTrackingStore{ratings: []store.MediaTracking{{MediaID: "42", Title: "Alien", Rating: 5}}}
	app := &App{sessions: sessions, tracking: tracking}

	request := httptest.NewRequest(http.MethodGet, "/api/tracking/ratings", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "Alien") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSetFavoriteHandlerAddsFavorite(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	favorites := &fakeFavoriteWriter{}
	app := &App{sessions: sessions, favorites: favorites}

	request := httptest.NewRequest(http.MethodPost, "/api/media/emby/favorite?itemId=42", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if favorites.userID != "user-1" || favorites.itemID != "42" || !favorites.favorite {
		t.Fatalf("userID = %q, itemID = %q, favorite = %v", favorites.userID, favorites.itemID, favorites.favorite)
	}
}

func TestSetFavoriteHandlerRemovesFavorite(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	favorites := &fakeFavoriteWriter{}
	app := &App{sessions: sessions, favorites: favorites}

	request := httptest.NewRequest(http.MethodDelete, "/api/media/emby/favorite?itemId=42", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if favorites.favorite {
		t.Fatal("expected favorite = false for DELETE")
	}
}

func TestSetFavoriteHandlerRequiresItemID(t *testing.T) {
	sessions := &memorySessionStore{identity: emby.Identity{UserID: "user-1"}}
	app := &App{sessions: sessions, favorites: &fakeFavoriteWriter{}}

	request := httptest.NewRequest(http.MethodPost, "/api/media/emby/favorite", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-id"})
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSetFavoriteHandlerRequiresSession(t *testing.T) {
	app := &App{sessions: &memorySessionStore{}, favorites: &fakeFavoriteWriter{}}

	request := httptest.NewRequest(http.MethodPost, "/api/media/emby/favorite?itemId=42", nil)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
