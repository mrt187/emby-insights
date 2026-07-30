package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrt187/EmbyInsights/internal/comingsoon"
	"github.com/mrt187/EmbyInsights/internal/config"
	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/seerr"
	"github.com/mrt187/EmbyInsights/internal/session"
	"github.com/mrt187/EmbyInsights/internal/store"
	"github.com/redis/go-redis/v9"
)

const sessionCookieName = "emby_insights_session"
const comingSoonCacheTTL = 15 * time.Minute

type App struct {
	database            *pgxpool.Pool
	redis               *redis.Client
	authenticator       emby.Authenticator
	statistics          emby.PersonalStatisticsReader
	watchTimeRank       emby.WatchTimeRankReader
	deviceStatistics    emby.DeviceStatisticsReader
	sessionStatistics   emby.SessionStatisticsReader
	avatars             emby.AvatarReader
	comingSoon          comingsoon.Reader
	newForYou           emby.NewForYouReader
	newForYouLibraryIDs []string
	continueWatching    emby.ContinueWatchingReader
	watched             emby.WatchedReader
	watchedLibraryIDs   []string
	completed           emby.CompletedReader
	profile             emby.ProfileReader
	requests            seerr.RequestsReader
	availableRequests   seerr.AvailableRequestsReader
	requestStats        seerr.RequestStatsReader
	discover            seerr.DiscoverReader
	embyMediaDetail     emby.MediaDetailReader
	seerrMediaDetail    seerr.MediaDetailReader
	seerrRequestCreator seerr.RequestCreator
	tracking            store.TrackingStore
	favorites           emby.FavoriteWriter
	sessions            session.Store
	cookieSecure        bool
	messages            store.MessageStore
	adminUserID         string
	directory           emby.UserDirectoryReader
	adminAvatars        emby.AdminAvatarReader
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
		database:            database,
		redis:               cache,
		authenticator:       embyClient,
		statistics:          embyClient,
		watchTimeRank:       embyClient,
		deviceStatistics:    embyClient,
		sessionStatistics:   embyClient,
		avatars:             embyClient,
		comingSoon:          comingsoon.NewClient(cfg.RadarrBaseURL, cfg.RadarrAPIKey, cfg.SonarrBaseURL, cfg.SonarrAPIKey, cfg.TmdbAPIKey, cfg.ComingSoonRegion, cfg.ComingSoonDaysAhead),
		newForYou:           embyClient,
		newForYouLibraryIDs: cfg.EmbyNewForYouLibraryIDs,
		continueWatching:    embyClient,
		watched:             embyClient,
		watchedLibraryIDs:   cfg.EmbyWatchedLibraryIDs,
		completed:           embyClient,
		profile:             embyClient,
		requests:            seerrClient,
		availableRequests:   seerrClient,
		requestStats:        seerrClient,
		discover:            seerrClient,
		embyMediaDetail:     embyClient,
		seerrMediaDetail:    seerrClient,
		seerrRequestCreator: seerrClient,
		tracking:            store.NewPostgresTrackingStore(database),
		favorites:           embyClient,
		sessions:            session.NewRedisStore(cache),
		cookieSecure:        cfg.CookieSecure,
		messages:            store.NewPostgresMessageStore(database),
		adminUserID:         cfg.AdminEmbyUserID,
		directory:           embyClient,
		adminAvatars:        embyClient,
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
	mux.HandleFunc("GET /api/me/profile", app.meProfile)
	mux.HandleFunc("GET /api/stats", app.stats)
	mux.HandleFunc("GET /api/stats/rank", app.watchTimeRankStats)
	mux.HandleFunc("GET /api/stats/devices", app.deviceStats)
	mux.HandleFunc("GET /api/stats/hours", app.hourStats)
	mux.HandleFunc("GET /api/stats/weekdays", app.weekdayStats)
	mux.HandleFunc("GET /api/stats/longest-session", app.longestSessionStats)
	mux.HandleFunc("GET /api/stats/most-active-day", app.mostActiveDayStats)
	mux.HandleFunc("GET /api/upcoming", app.upcomingItems)
	mux.HandleFunc("GET /api/in-cinemas", app.inCinemaItems)
	mux.HandleFunc("GET /api/requests", app.myRequests)
	mux.HandleFunc("GET /api/requests/available", app.availableRequestItems)
	mux.HandleFunc("GET /api/requests/total", app.requestsTotal)
	mux.HandleFunc("GET /api/new-for-you", app.newForYouItems)
	mux.HandleFunc("GET /api/continue-watching", app.continueWatchingItems)
	mux.HandleFunc("GET /api/watched-movies", app.watchedMovies)
	mux.HandleFunc("GET /api/watched-series", app.watchedSeries)
	mux.HandleFunc("GET /api/completed-movies", app.completedMovies)
	mux.HandleFunc("GET /api/completed-series", app.completedSeries)
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
	mux.HandleFunc("GET /api/discover/search", app.discoverSearch)
	mux.HandleFunc("GET /api/media/emby", app.embyMediaDetailHandler)
	mux.HandleFunc("GET /api/media/seerr", app.seerrMediaDetailHandler)
	mux.HandleFunc("POST /api/media/seerr/request", app.createSeerrRequestHandler)
	mux.HandleFunc("GET /api/tracking", app.getTracking)
	mux.HandleFunc("PUT /api/tracking", app.upsertTracking)
	mux.HandleFunc("GET /api/tracking/watchlist", app.trackingWatchlist)
	mux.HandleFunc("GET /api/tracking/ratings", app.trackingRatings)
	mux.HandleFunc("POST /api/media/emby/favorite", app.setFavoriteHandler(true))
	mux.HandleFunc("DELETE /api/media/emby/favorite", app.setFavoriteHandler(false))
	mux.HandleFunc("GET /api/messages", app.getMessages)
	mux.HandleFunc("POST /api/messages", app.sendMessage)
	mux.HandleFunc("POST /api/messages/read", app.markOwnThreadRead)
	mux.HandleFunc("GET /api/messages/unread-count", app.unreadMessageCount)
	mux.HandleFunc("GET /api/admin/messages/threads", app.adminMessageThreads)
	mux.HandleFunc("GET /api/admin/messages/thread", app.adminMessageThread)
	mux.HandleFunc("POST /api/admin/messages/thread", app.adminSendMessage)
	mux.HandleFunc("POST /api/admin/messages/thread/read", app.adminMarkThreadRead)
	mux.HandleFunc("GET /api/admin/users", app.adminUserDirectory)
	mux.HandleFunc("GET /api/admin/users/avatar", app.adminUserAvatar)
	mux.HandleFunc("POST /api/admin/messages/broadcast", app.adminBroadcastMessage)
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
	respondJSON(writer, http.StatusOK, app.identityProfile(identity))
}

func (app *App) me(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	respondJSON(writer, http.StatusOK, app.identityProfile(identity))
}

func (app *App) meProfile(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	userProfile, err := app.profile.UserProfile(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "user profile is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, userProfile)
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

func (app *App) watchTimeRankStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if app.watchTimeRank == nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "watch-time rank is unavailable"})
		return
	}
	rank, err := app.watchTimeRank.WatchTimeRank(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "watch-time rank is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, rank)
}

func (app *App) deviceStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	devices, err := app.deviceStatistics.DeviceWatchTimes(request.Context(), identity.UserID, period)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "device statistics are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(devices))
}

func (app *App) hourStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	hours, err := app.sessionStatistics.HourWatchTimes(request.Context(), identity.UserID, period)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "hour statistics are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(hours))
}

func (app *App) weekdayStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	weekdays, err := app.sessionStatistics.WeekdayWatchTimes(request.Context(), identity.UserID, period)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "weekday statistics are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(weekdays))
}

func (app *App) longestSessionStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	session, found, err := app.sessionStatistics.LongestSession(request.Context(), identity.UserID, period)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "longest-session statistics are unavailable"})
		return
	}
	if !found {
		respondJSON(writer, http.StatusOK, nil)
		return
	}
	respondJSON(writer, http.StatusOK, session)
}

func (app *App) mostActiveDayStats(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	day, found, err := app.sessionStatistics.MostActiveDay(request.Context(), identity.UserID, period)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "most-active-day statistics are unavailable"})
		return
	}
	if !found {
		respondJSON(writer, http.StatusOK, nil)
		return
	}
	respondJSON(writer, http.StatusOK, day)
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
	items, err := app.cachedComingSoonItems(request.Context(), "upcoming", app.comingSoon.Upcoming)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "upcoming releases are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) inCinemaItems(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	items, err := app.cachedComingSoonItems(request.Context(), "in-cinemas", app.comingSoon.InCinemas)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "cinema releases are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

// cachedComingSoonItems keeps calendar and TMDB lookups off the request path
// for a short period. The upstream services remain the source of truth; a
// cache failure must never make the dashboard unavailable.
func (app *App) cachedComingSoonItems(ctx context.Context, kind string, read func(context.Context) ([]comingsoon.Item, error)) ([]comingsoon.Item, error) {
	if app.redis != nil {
		if value, err := app.redis.Get(ctx, "comingsoon:"+kind).Result(); err == nil {
			var items []comingsoon.Item
			if json.Unmarshal([]byte(value), &items) == nil {
				return items, nil
			}
		}
	}
	items, err := read(ctx)
	if err != nil {
		return nil, err
	}
	if app.redis != nil {
		if value, err := json.Marshal(items); err == nil {
			_ = app.redis.Set(ctx, "comingsoon:"+kind, value, comingSoonCacheTTL).Err()
		}
	}
	return items, nil
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

// availableRequestWindow bounds how long a title stays newsworthy on the
// "Jetzt relevant" tile after Seerr recorded it as added to the library.
const availableRequestWindow = 7 * 24 * time.Hour

func (app *App) availableRequestItems(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.availableRequests.AvailableRequests(request.Context(), identity.UserID, time.Now().Add(-availableRequestWindow))
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "available requests are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) requestsTotal(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	stats, err := app.requestStats.RequestStats(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "request stats are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, stats)
}

func (app *App) discoverSearch(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	query := request.URL.Query().Get("query")
	if query == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "query is required"})
		return
	}
	items, err := app.discover.Search(request.Context(), query)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "search is unavailable"})
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

func (app *App) createSeerrRequestHandler(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}

	var input struct {
		MediaType string `json:"mediaType"`
		TmdbID    int    `json:"tmdbId"`
		Seasons   []int  `json:"seasons"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<16)
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if input.MediaType != "movie" && input.MediaType != "tv" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "mediaType must be movie or tv"})
		return
	}
	if input.TmdbID <= 0 {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "tmdbId is required"})
		return
	}

	if err := app.seerrRequestCreator.CreateRequest(request.Context(), identity.UserID, input.MediaType, input.TmdbID, input.Seasons); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "creating the Seerr request failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (app *App) getTracking(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	mediaSource := request.URL.Query().Get("source")
	mediaID := request.URL.Query().Get("id")
	if mediaSource == "" || mediaID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "source and id are required"})
		return
	}

	entry, found, err := app.tracking.Get(request.Context(), identity.UserID, mediaSource, mediaID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "tracking is unavailable"})
		return
	}
	if !found {
		respondJSON(writer, http.StatusOK, store.MediaTracking{MediaSource: mediaSource, MediaID: mediaID})
		return
	}
	respondJSON(writer, http.StatusOK, entry)
}

func (app *App) upsertTracking(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}

	var entry store.MediaTracking
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<16)
	if err := json.NewDecoder(request.Body).Decode(&entry); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if entry.MediaSource == "" || entry.MediaID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "mediaSource and mediaId are required"})
		return
	}
	if entry.Rating < 0 || entry.Rating > 5 {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "rating must be between 0 and 5"})
		return
	}

	if err := app.tracking.Upsert(request.Context(), identity.UserID, entry); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "saving tracking failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (app *App) trackingWatchlist(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.tracking.Watchlist(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "watchlist is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) trackingRatings(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.tracking.Ratings(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "ratings are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

const maxMessageBodyLength = 4000

// requireAdmin rejects the request unless ADMIN_EMBY_USER_ID is configured
// and matches the caller. Chat is a closed loop between each user and this
// one fixed admin identity — there is no broader role system.
func (app *App) requireAdmin(writer http.ResponseWriter, identity emby.Identity) bool {
	if app.adminUserID == "" || identity.UserID != app.adminUserID {
		respondJSON(writer, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return false
	}
	return true
}

func decodeMessageBody(writer http.ResponseWriter, request *http.Request) (string, bool) {
	var input struct {
		Body string `json:"body"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<16)
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return "", false
	}
	trimmed := strings.TrimSpace(input.Body)
	if trimmed == "" || len(trimmed) > maxMessageBodyLength {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "body must be 1-4000 characters"})
		return "", false
	}
	return trimmed, true
}

// decodeAdminMessageBody additionally accepts a displayName, sent along by
// the frontend when the admin starts a brand new thread from the user
// picker (see adminUserDirectory) — there is no other way to learn a user's
// name before they have sent their own first message.
func decodeAdminMessageBody(writer http.ResponseWriter, request *http.Request) (body, displayName string, ok bool) {
	var input struct {
		Body        string `json:"body"`
		DisplayName string `json:"displayName"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<16)
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return "", "", false
	}
	trimmed := strings.TrimSpace(input.Body)
	if trimmed == "" || len(trimmed) > maxMessageBodyLength {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "body must be 1-4000 characters"})
		return "", "", false
	}
	return trimmed, strings.TrimSpace(input.DisplayName), true
}

// getMessages returns the caller's own thread with the admin. The admin has
// no thread of their own — they use the /api/admin/messages/* endpoints.
func (app *App) getMessages(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if identity.UserID == app.adminUserID {
		respondJSON(writer, http.StatusForbidden, map[string]string{"error": "the admin account has no thread of its own"})
		return
	}
	messages, err := app.messages.Thread(request.Context(), identity.UserID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "messages are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(messages))
}

func (app *App) sendMessage(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if identity.UserID == app.adminUserID {
		respondJSON(writer, http.StatusForbidden, map[string]string{"error": "the admin account has no thread of its own"})
		return
	}
	body, ok := decodeMessageBody(writer, request)
	if !ok {
		return
	}
	if err := app.messages.Send(request.Context(), identity.UserID, identity.DisplayName, body, false); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "sending the message failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (app *App) markOwnThreadRead(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if err := app.messages.MarkRead(request.Context(), identity.UserID, true); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "updating messages failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// unreadMessageCount backs the notification bell. For a regular user it is
// how many admin replies they haven't read; for the admin it is how many
// user messages are unread across every thread.
func (app *App) unreadMessageCount(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	var count int
	var err error
	if identity.UserID == app.adminUserID {
		count, err = app.messages.UnreadCountForAdmin(request.Context())
	} else {
		count, err = app.messages.UnreadCountForUser(request.Context(), identity.UserID)
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "unread count is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, map[string]int{"count": count})
}

func (app *App) adminMessageThreads(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	threads, err := app.messages.Threads(request.Context())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "threads are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(threads))
}

func (app *App) adminMessageThread(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	userID := request.URL.Query().Get("userId")
	if userID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}
	messages, err := app.messages.Thread(request.Context(), userID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "messages are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(messages))
}

func (app *App) adminSendMessage(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	userID := request.URL.Query().Get("userId")
	if userID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}
	body, displayName, ok := decodeAdminMessageBody(writer, request)
	if !ok {
		return
	}
	if err := app.messages.Send(request.Context(), userID, displayName, body, true); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "sending the message failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// adminUserDirectory lists every Emby user except the admin, for the "start
// a new chat" picker — including people who have never opened this app, so
// the admin can reach out first if they want to.
func (app *App) adminUserDirectory(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	users, err := app.directory.Users(request.Context())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "the user directory is unavailable"})
		return
	}
	contacts := make([]map[string]string, 0, len(users))
	for _, user := range users {
		if user.ID == app.adminUserID {
			continue
		}
		contacts = append(contacts, map[string]string{"id": user.ID, "name": user.Name})
	}
	sort.Slice(contacts, func(i, j int) bool { return contacts[i]["name"] < contacts[j]["name"] })
	respondJSON(writer, http.StatusOK, contacts)
}

func (app *App) adminUserAvatar(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	userID := request.URL.Query().Get("userId")
	if userID == "" || !validEmbyItemID.MatchString(userID) {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}
	image, err := app.adminAvatars.UserPrimaryImageByID(request.Context(), userID)
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

// adminBroadcastMessage sends one message to every Emby user's thread at
// once (e.g. a maintenance notice) — the admin's only way to reach everyone
// without visiting each thread individually.
func (app *App) adminBroadcastMessage(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	body, ok := decodeMessageBody(writer, request)
	if !ok {
		return
	}
	users, err := app.directory.Users(request.Context())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "the user directory is unavailable"})
		return
	}
	sent := 0
	for _, user := range users {
		if user.ID == app.adminUserID {
			continue
		}
		if err := app.messages.Send(request.Context(), user.ID, user.Name, body, true); err != nil {
			respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "sending the broadcast failed partway through"})
			return
		}
		sent++
	}
	respondJSON(writer, http.StatusOK, map[string]int{"count": sent})
}

func (app *App) adminMarkThreadRead(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, identity) {
		return
	}
	userID := request.URL.Query().Get("userId")
	if userID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}
	if err := app.messages.MarkRead(request.Context(), userID, false); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "updating messages failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// validEmbyItemID matches the shapes Emby actually uses for item IDs — hex
// GUIDs (with or without dashes) and numeric IDs. Anything else is rejected
// before it can reach SetFavorite, which interpolates the ID into a URL sent
// with the admin API key.
var validEmbyItemID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// setFavoriteHandler returns a handler for both the POST and DELETE Emby
// favorite routes, since they differ only in which way the toggle goes.
func (app *App) setFavoriteHandler(favorite bool) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		identity, ok := app.identityFromRequest(writer, request)
		if !ok {
			return
		}
		itemID := request.URL.Query().Get("itemId")
		if itemID == "" {
			respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "itemId is required"})
			return
		}
		if !validEmbyItemID.MatchString(itemID) {
			respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "itemId is malformed"})
			return
		}
		if err := app.favorites.SetFavorite(request.Context(), identity.UserID, itemID, favorite); err != nil {
			respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "updating the Emby favorite failed"})
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
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

func (app *App) completedMovies(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	from, to := emby.PeriodBounds(period, time.Now())
	items, err := app.completed.CompletedMovies(request.Context(), identity.UserID, app.watchedLibraryIDs, from, to)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "completed movies are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) completedSeries(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	period, ok := parsePeriod(writer, request)
	if !ok {
		return
	}
	from, to := emby.PeriodBounds(period, time.Now())
	items, err := app.completed.CompletedSeries(request.Context(), identity.UserID, app.watchedLibraryIDs, from, to)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "completed series are unavailable"})
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

func (app *App) identityProfile(identity emby.Identity) map[string]any {
	return map[string]any{
		"id":      identity.UserID,
		"name":    identity.DisplayName,
		"isAdmin": app.adminUserID != "" && identity.UserID == app.adminUserID,
	}
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
