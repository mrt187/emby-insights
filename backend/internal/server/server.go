package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrt187/EmbyInsights/internal/config"
	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/seerr"
	"github.com/mrt187/EmbyInsights/internal/session"
	"github.com/redis/go-redis/v9"
)

const sessionCookieName = "emby_insights_session"

type App struct {
	database             *pgxpool.Pool
	redis                *redis.Client
	authenticator        emby.Authenticator
	statistics           emby.PersonalStatisticsReader
	avatars              emby.AvatarReader
	upcoming             emby.UpcomingReader
	comingSoonLibraryIDs []string
	newForYou            emby.NewForYouReader
	newForYouLibraryIDs  []string
	continueWatching     emby.ContinueWatchingReader
	watched              emby.WatchedReader
	watchedLibraryIDs    []string
	requests             seerr.RequestsReader
	discover             seerr.DiscoverReader
	embyMediaDetail      emby.MediaDetailReader
	seerrMediaDetail     seerr.MediaDetailReader
	sessions             session.Store
	cookieSecure         bool
}

func New(cfg config.Config) (*App, error) {
	database, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	options, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		database.Close()
		return nil, err
	}
	cache := redis.NewClient(options)
	embyClient := emby.NewClient(cfg.EmbyBaseURL, cfg.EmbyDeviceID, cfg.EmbyAdminAPIKey)
	seerrClient := seerr.NewClient(cfg.SeerrBaseURL, cfg.SeerrAPIKey)
	return &App{
		database:             database,
		redis:                cache,
		authenticator:        embyClient,
		statistics:           embyClient,
		avatars:              embyClient,
		upcoming:             embyClient,
		comingSoonLibraryIDs: cfg.EmbyComingSoonLibraryIDs,
		newForYou:            embyClient,
		newForYouLibraryIDs:  cfg.EmbyNewForYouLibraryIDs,
		continueWatching:     embyClient,
		watched:              embyClient,
		watchedLibraryIDs:    cfg.EmbyWatchedLibraryIDs,
		requests:             seerrClient,
		discover:             seerrClient,
		embyMediaDetail:      embyClient,
		seerrMediaDetail:     seerrClient,
		sessions:             session.NewRedisStore(cache),
		cookieSecure:         cfg.CookieSecure,
	}, nil
}

func (app *App) Close() { app.redis.Close(); app.database.Close() }

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", app.ready)
	mux.HandleFunc("POST /api/auth/emby/login", app.login)
	mux.HandleFunc("POST /api/auth/logout", app.logout)
	mux.HandleFunc("GET /api/me", app.me)
	mux.HandleFunc("GET /api/me/avatar", app.avatar)
	mux.HandleFunc("GET /api/stats", app.stats)
	mux.HandleFunc("GET /api/upcoming", app.upcomingItems)
	mux.HandleFunc("GET /api/requests", app.myRequests)
	mux.HandleFunc("GET /api/new-for-you", app.newForYouItems)
	mux.HandleFunc("GET /api/continue-watching", app.continueWatchingItems)
	mux.HandleFunc("GET /api/watched-movies", app.watchedMovies)
	mux.HandleFunc("GET /api/watched-series", app.watchedSeries)
	mux.HandleFunc("GET /api/discover/trending", app.discoverHandler(func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.Trending(ctx)
	}))
	mux.HandleFunc("GET /api/discover/movies/popular", app.discoverHandler(func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.PopularMovies(ctx)
	}))
	mux.HandleFunc("GET /api/discover/movies/upcoming", app.discoverHandler(func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.UpcomingMovies(ctx)
	}))
	mux.HandleFunc("GET /api/discover/series/popular", app.discoverHandler(func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.PopularSeries(ctx)
	}))
	mux.HandleFunc("GET /api/discover/series/upcoming", app.discoverHandler(func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.UpcomingSeries(ctx)
	}))
	mux.HandleFunc("GET /api/media/emby", app.embyMediaDetailHandler)
	mux.HandleFunc("GET /api/media/seerr", app.seerrMediaDetailHandler)
	return mux
}

func health(writer http.ResponseWriter, _ *http.Request) {
	respondJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (app *App) ready(writer http.ResponseWriter, request *http.Request) {
	context, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()
	if err := app.database.Ping(context); err != nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}
	if err := app.redis.Ping(context).Err(); err != nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "cache unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func (app *App) login(writer http.ResponseWriter, request *http.Request) {
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil || input.Username == "" || input.Password == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}
	identity, err := app.authenticator.Authenticate(request.Context(), emby.Credentials{Username: input.Username, Password: input.Password})
	if errors.Is(err, emby.ErrInvalidCredentials) {
		respondJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "Emby is unavailable"})
		return
	}
	sessionID, err := app.sessions.Create(request.Context(), identity)
	if err != nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "session storage is unavailable"})
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: sessionID, Path: "/", MaxAge: int(session.Duration.Seconds()), HttpOnly: true, Secure: app.cookieSecure, SameSite: http.SameSiteLaxMode})
	respondJSON(writer, http.StatusOK, profile(identity))
}

func (app *App) me(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	respondJSON(writer, http.StatusOK, profile(identity))
}

func (app *App) stats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	statistics, err := app.statistics.PersonalWatchTime(request.Context(), identity.UserID, period)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "personal statistics are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, statistics)
}

func parsePeriod(writer http.ResponseWriter, request *http.Request) (string, bool) {
	period := request.URL.Query().Get("period")
	if period == "" {
		period = "week"
	}
	if period != "week" && period != "month" && period != "year" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "period must be week, month or year"})
		return "", false
	}
	return period, true
}

func (app *App) upcomingItems(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	items, err := app.upcoming.Upcoming(request.Context(), app.comingSoonLibraryIDs)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "upcoming releases are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) myRequests(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.requests.Requests(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "requests are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

// discoverHandler builds a handler for one Seerr discover list. Discover
// data is not personal, but the endpoint still requires a session like the
// rest of the API.
func (app *App) discoverHandler(read func(context.Context, seerr.DiscoverReader) ([]seerr.DiscoverItem, error)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := app.identityFromRequest(writer, request); !ok {
			return
		}
		items, err := read(request.Context(), app.discover)
		if err != nil {
			respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "discover list is unavailable"})
			return
		}
		respondJSON(writer, http.StatusOK, orEmpty(items))
	}
}

func (app *App) embyMediaDetailHandler(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	itemID := request.URL.Query().Get("id")
	if itemID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	detail, err := app.embyMediaDetail.EmbyMediaDetail(request.Context(), identity.UserID, itemID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "media detail is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, detail)
}

func (app *App) seerrMediaDetailHandler(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	mediaType := request.URL.Query().Get("mediaType")
	tmdbID, parseErr := strconv.Atoi(request.URL.Query().Get("id"))
	if (mediaType != "movie" && mediaType != "tv") || parseErr != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "mediaType must be movie or tv, and id must be a TMDB id"})
		return
	}
	detail, err := app.seerrMediaDetail.MediaDetail(request.Context(), mediaType, tmdbID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "media detail is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, detail)
}

func (app *App) newForYouItems(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.newForYou.NewForYou(request.Context(), identity.UserID, app.newForYouLibraryIDs)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "new items are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) continueWatchingItems(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.continueWatching.ContinueWatching(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "continue watching is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) watchedMovies(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.watched.WatchedMovies(request.Context(), identity.UserID, app.watchedLibraryIDs)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "watched movies are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) watchedSeries(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.watched.WatchedSeries(request.Context(), identity.UserID, app.watchedLibraryIDs)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "watched series are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

// orEmpty ensures a nil slice serializes as `[]` instead of `null`, so the
// frontend never has to special-case a missing array.
func orEmpty[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func (app *App) logout(writer http.ResponseWriter, request *http.Request) {
	if cookie, err := request.Cookie(sessionCookieName); err == nil {
		_ = app.sessions.Delete(request.Context(), cookie.Value)
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: app.cookieSecure, SameSite: http.SameSiteLaxMode})
	writer.WriteHeader(http.StatusNoContent)
}

func (app *App) identityFromRequest(writer http.ResponseWriter, request *http.Request) (emby.Identity, bool) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil {
		respondJSON(writer, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return emby.Identity{}, false
	}
	identity, err := app.sessions.Get(request.Context(), cookie.Value)
	if err != nil {
		respondJSON(writer, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return emby.Identity{}, false
	}
	return identity, true
}

func profile(identity emby.Identity) map[string]string {
	return map[string]string{"id": identity.UserID, "name": identity.DisplayName}
}

func respondJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}

func (app *App) avatar(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	image, err := app.avatars.UserPrimaryImage(request.Context(), identity)
	if errors.Is(err, emby.ErrPrimaryImageUnavailable) {
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "profile image is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", image.ContentType)
	writer.Header().Set("Cache-Control", "private, max-age=3600")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(image.Data)
}
