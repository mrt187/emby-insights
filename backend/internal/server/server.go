package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mrt187/EmbyInsights/internal/artwork"
	"log"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrt187/EmbyInsights/internal/appconfig"
	"github.com/mrt187/EmbyInsights/internal/comingsoon"
	"github.com/mrt187/EmbyInsights/internal/config"
	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/mrt187/EmbyInsights/internal/omdb"
	"github.com/mrt187/EmbyInsights/internal/secretbox"
	"github.com/mrt187/EmbyInsights/internal/seerr"
	"github.com/mrt187/EmbyInsights/internal/session"
	"github.com/mrt187/EmbyInsights/internal/store"
	"github.com/redis/go-redis/v9"
)

const sessionCookieName = "emby_insights_session"
const comingSoonCacheTTL = 15 * time.Minute
const discoverCacheTTL = 1 * time.Hour
const statsCacheTTL = 5 * time.Minute
const requestsCacheTTL = 5 * time.Minute
const topRatedLimit = 15

// imageCacheTTL is long because the cache key includes Emby's image tag,
// which changes whenever the underlying image does — a cache hit is by
// definition still current.
const imageCacheTTL = 7 * 24 * time.Hour

// ConfigStore is the subset of appconfig.Store that App depends on, kept as
// an interface (like every other store dependency here — MessageStore,
// TrackingStore, etc.) so admin-gating and features-matrix behavior can be
// unit-tested with an in-memory fake instead of a real Postgres instance.
type ConfigStore interface {
	Get(ctx context.Context) (appconfig.Settings, error)
	Update(ctx context.Context, settings appconfig.Settings) error
	EnsureDeviceID(ctx context.Context) (string, error)
	ClaimAdminOwner(ctx context.Context, candidateUserID string) (string, error)
	CurrentAdminOwner(ctx context.Context) (string, error)
	SeedFromEnvIfEmpty(ctx context.Context) error
}

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
	continueWatching    emby.ContinueWatchingReader
	watched             emby.WatchedReader
	seriesInProgress    emby.SeriesInProgressReader
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
	activity            store.ActivityStore
	favorites           emby.FavoriteWriter
	playedItems         emby.PlayedWriter
	sessions            session.Store
	cookieSecure        bool
	messages            store.MessageStore
	directory           emby.UserDirectoryReader
	adminAvatars        emby.AdminAvatarReader
	embyClient          *emby.Client
	appconfig           ConfigStore
	live                *liveConfig
	imageFetchClient    *http.Client
	trustedProxies      []*net.IPNet
	loginLimiters       map[string]*loginRateLimiter
	loginLimitersMu     sync.Mutex
	loginLimitersSwept  time.Time
	// loginSprayDelay overrides loginSprayDelay for tests; zero means the const.
	loginSprayDelay time.Duration
}

type loginRateLimiter struct {
	attempts []time.Time
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

	box, err := secretbox.New(cfg.AppEncryptionKey)
	if err != nil {
		database.Close()
		return nil, err
	}
	appconfigStore := appconfig.NewStore(database, box)

	ctx := context.Background()
	if err := appconfigStore.SeedFromEnvIfEmpty(ctx); err != nil {
		log.Printf("warning: seeding app_config from legacy env vars failed: %v", err)
	}
	settings, err := appconfigStore.Get(ctx)
	if err != nil {
		// A settings-read failure must never block startup — every
		// integration it configures is already optional/nil-safe, so the
		// dashboard still boots with everything but Emby itself disabled.
		log.Printf("warning: reading app_config failed, starting with integrations disabled: %v", err)
	}
	deviceID, err := appconfigStore.EnsureDeviceID(ctx)
	if err != nil {
		log.Printf("warning: generating Emby device id failed: %v", err)
	}

	embyClient := emby.NewClient(cfg.EmbyBaseURL, deviceID, cfg.EmbyAdminAPIKey)

	live := &liveConfig{}
	live.set(
		seerr.NewClient(settings.Seerr.EnabledBaseURL(), settings.Seerr.EnabledAPIKey()),
		comingsoon.NewClient(settings.Radarr.EnabledBaseURL(), settings.Radarr.EnabledAPIKey(), settings.Sonarr.EnabledBaseURL(), settings.Sonarr.EnabledAPIKey(), settings.TMDB.EnabledAPIKey(), settings.ComingSoonRegion, settings.ComingSoonDaysAhead),
		omdb.NewClient(settings.OMDB.EnabledAPIKey()),
		settings.ComingSoonRegion,
		settings.NewForYouLibraryIDs,
		settings.WatchedLibraryIDs,
	)
	seerrFacade := liveSeerr{live: live}
	comingSoonFacade := liveComingSoon{live: live}

	app := &App{
		database:            database,
		redis:               cache,
		authenticator:       embyClient,
		statistics:          embyClient,
		watchTimeRank:       embyClient,
		deviceStatistics:    embyClient,
		sessionStatistics:   embyClient,
		avatars:             embyClient,
		comingSoon:          comingSoonFacade,
		newForYou:           embyClient,
		continueWatching:    embyClient,
		watched:             embyClient,
		seriesInProgress:    embyClient,
		completed:           embyClient,
		profile:             embyClient,
		requests:            seerrFacade,
		availableRequests:   seerrFacade,
		requestStats:        seerrFacade,
		discover:            seerrFacade,
		embyMediaDetail:     embyClient,
		seerrMediaDetail:    seerrFacade,
		seerrRequestCreator: seerrFacade,
		tracking:            store.NewPostgresTrackingStore(database),
		activity:            store.NewPostgresActivityStore(database),
		favorites:           embyClient,
		playedItems:         embyClient,
		sessions:            session.NewRedisStore(cache),
		cookieSecure:        cfg.CookieSecure,
		messages:            store.NewPostgresMessageStore(database),
		directory:           embyClient,
		adminAvatars:        embyClient,
		embyClient:          embyClient,
		appconfig:           appconfigStore,
		live:                live,
		imageFetchClient:    newImageFetchClient(),
		trustedProxies:      parseTrustedProxies(cfg.TrustedProxies),
		loginLimiters:       make(map[string]*loginRateLimiter),
	}
	// Redis persists across restarts/redeploys (appendonly file on the same
	// volume as everything else), so a fixed integration response (e.g. the
	// comingsoon poster URL scheme) can otherwise keep being served from a
	// pre-fix cache entry for up to its full TTL after the new binary is
	// already running.
	app.invalidateIntegrationCaches(ctx)
	return app, nil
}

// applySettings persists new setup-wizard settings and immediately swaps the
// live Seerr/Radarr/Sonarr/TMDB clients and library selections in place, so
// the admin never has to restart the container after a Verwaltung change.
func (app *App) applySettings(ctx context.Context, settings appconfig.Settings) error {
	if err := app.appconfig.Update(ctx, settings); err != nil {
		return err
	}
	// Update() merges an empty APIKey field with the previously stored key
	// before persisting (so "save without retyping the key" keeps it) — but
	// that merge happens on Update's own local copy and never modifies the
	// settings variable here. Re-reading what was actually persisted, rather
	// than reusing the possibly-still-empty input, is what the live clients
	// must be built from; otherwise saving without retyping the key silently
	// disables the integration at runtime even though the database (and the
	// admin GET view) still show it fully configured.
	persisted, err := app.appconfig.Get(ctx)
	if err != nil {
		return err
	}
	app.live.set(
		seerr.NewClient(persisted.Seerr.EnabledBaseURL(), persisted.Seerr.EnabledAPIKey()),
		comingsoon.NewClient(persisted.Radarr.EnabledBaseURL(), persisted.Radarr.EnabledAPIKey(), persisted.Sonarr.EnabledBaseURL(), persisted.Sonarr.EnabledAPIKey(), persisted.TMDB.EnabledAPIKey(), persisted.ComingSoonRegion, persisted.ComingSoonDaysAhead),
		omdb.NewClient(persisted.OMDB.EnabledAPIKey()),
		persisted.ComingSoonRegion,
		persisted.NewForYouLibraryIDs,
		persisted.WatchedLibraryIDs,
	)
	// Requests/discover/comingsoon responses cached before this change (e.g.
	// an empty result cached while Seerr was misconfigured) would otherwise
	// keep being served for up to their full TTL — a Verwaltung change must
	// take effect immediately, not "eventually".
	app.invalidateIntegrationCaches(ctx)
	return nil
}

func (app *App) invalidateIntegrationCaches(ctx context.Context) {
	if app.redis == nil {
		return
	}
	for _, pattern := range []string{"requests:*", "discover:*", "comingsoon:*"} {
		keys, err := app.redis.Keys(ctx, pattern).Result()
		if err != nil || len(keys) == 0 {
			continue
		}
		_ = app.redis.Del(ctx, keys...).Err()
	}
}

func (app *App) Close() { app.redis.Close(); app.database.Close() }

// contentSecurityPolicy is the second line of defence behind the image
// content-type checks: even if something non-image were ever served from our
// own origin, this stops it from loading scripts or phoning home.
//
// 'unsafe-inline' is present for styles only — the frontend build ships
// inline <style> blocks and inline style attributes. Scripts get no such
// exemption. img-src allows data: for the inline placeholder artwork and
// blob: for canvas-rendered chart exports.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self' data:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", app.ready)
	mux.HandleFunc("POST /api/auth/emby/login", app.login)
	// Unauthenticated on purpose: the login screen renders before any session
	// exists and still has to label itself in the configured language.
	mux.HandleFunc("GET /api/language", app.uiLanguageHandler)
	mux.HandleFunc("POST /api/auth/logout", app.logout)
	mux.HandleFunc("GET /api/me", app.me)
	mux.HandleFunc("GET /api/me/avatar", app.avatar)
	mux.HandleFunc("GET /api/images", app.itemImage)
	mux.HandleFunc("GET /api/artwork", app.artworkImage)
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
	mux.HandleFunc("GET /api/top-rated", app.topRatedHandler)
	mux.HandleFunc("GET /api/watched-movies", app.watchedMovies)
	mux.HandleFunc("GET /api/watched-series", app.watchedSeries)
	mux.HandleFunc("GET /api/series-in-progress", app.seriesInProgressHandler)
	mux.HandleFunc("GET /api/completed-movies", app.completedMovies)
	mux.HandleFunc("GET /api/completed-series", app.completedSeries)
	mux.HandleFunc("GET /api/discover/trending", app.discoverHandler("discover:trending", func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.Trending(ctx)
	}))
	mux.HandleFunc("GET /api/discover/movies/popular", app.discoverHandler("discover:movies-popular", func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.PopularMovies(ctx)
	}))
	mux.HandleFunc("GET /api/discover/movies/upcoming", app.discoverHandler("discover:movies-upcoming", func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.UpcomingMovies(ctx)
	}))
	mux.HandleFunc("GET /api/discover/series/popular", app.discoverHandler("discover:series-popular", func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.PopularSeries(ctx)
	}))
	mux.HandleFunc("GET /api/discover/series/upcoming", app.discoverHandler("discover:series-upcoming", func(ctx context.Context, discover seerr.DiscoverReader) ([]seerr.DiscoverItem, error) {
		return discover.UpcomingSeries(ctx)
	}))
	mux.HandleFunc("GET /api/discover/search", app.discoverSearch)
	mux.HandleFunc("GET /api/discover/provider", app.discoverByProvider)
	mux.HandleFunc("GET /api/media/emby", app.embyMediaDetailHandler)
	mux.HandleFunc("GET /api/media/seerr", app.seerrMediaDetailHandler)
	mux.HandleFunc("GET /api/media/comingsoon", app.comingSoonMediaDetailHandler)
	mux.HandleFunc("POST /api/media/seerr/request", app.createSeerrRequestHandler)
	mux.HandleFunc("GET /api/tracking", app.getTracking)
	mux.HandleFunc("PUT /api/tracking", app.upsertTracking)
	mux.HandleFunc("GET /api/tracking/watchlist", app.trackingWatchlist)
	mux.HandleFunc("GET /api/tracking/ratings", app.trackingRatings)
	mux.HandleFunc("GET /api/tracking/poster", app.trackingPosterImage)
	mux.HandleFunc("POST /api/media/emby/favorite", app.setFavoriteHandler(true))
	mux.HandleFunc("DELETE /api/media/emby/favorite", app.setFavoriteHandler(false))
	mux.HandleFunc("POST /api/media/emby/played", app.setPlayedHandler(true))
	mux.HandleFunc("GET /api/messages", app.getMessages)
	mux.HandleFunc("POST /api/messages", app.sendMessage)
	mux.HandleFunc("POST /api/messages/read", app.markOwnThreadRead)
	mux.HandleFunc("GET /api/messages/unread-count", app.unreadMessageCount)
	mux.HandleFunc("GET /api/messages/admin-avatar", app.adminAvatarForUser)
	mux.HandleFunc("GET /api/admin/messages/threads", app.adminMessageThreads)
	mux.HandleFunc("GET /api/admin/messages/thread", app.adminMessageThread)
	mux.HandleFunc("POST /api/admin/messages/thread", app.adminSendMessage)
	mux.HandleFunc("POST /api/admin/messages/thread/read", app.adminMarkThreadRead)
	mux.HandleFunc("DELETE /api/admin/messages/thread", app.adminDeleteThread)
	mux.HandleFunc("GET /api/admin/users", app.adminUserDirectory)
	mux.HandleFunc("GET /api/admin/users/avatar", app.adminUserAvatar)
	mux.HandleFunc("POST /api/admin/messages/broadcast", app.adminBroadcastMessage)
	mux.HandleFunc("GET /api/admin/libraries", app.adminLibraries)
	mux.HandleFunc("GET /api/admin/settings", app.adminGetSettings)
	mux.HandleFunc("PUT /api/admin/settings", app.adminPutSettings)
	mux.HandleFunc("GET /api/admin/debug/live", app.adminDebugLive)
	mux.HandleFunc("GET /api/admin/activity", app.adminActivity)
	return securityHeaders(mux)
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

// Login throttling counts failed attempts on two keys. The per (username, IP)
// key is the only one that ever refuses a request: it is scoped to a single
// source, so exhausting it can never affect anybody else.
//
// The per-username key catches one account being guessed from many addresses,
// but it deliberately does not deny — it delays. A username-scoped *block*
// would hand any attacker a lockout: 20 wrong passwords in a minute and the
// real owner is out too. Slowing every further attempt down instead makes
// spraying expensive while the owner always gets in, just a little later.
const (
	loginFailuresPerUserAndIP = 5
	loginFailuresPerUser      = 20
	loginFailureWindow        = time.Minute
	loginSprayDelay           = 2 * time.Second
)

// parseTrustedProxies turns the configured CIDR list into networks whose
// X-Forwarded-For header may be believed. Anything unparseable is dropped
// rather than fatal: a typo in the deployment env should degrade to "trust
// nobody" instead of refusing to boot.
func parseTrustedProxies(entries []string) []*net.IPNet {
	networks := make([]*net.IPNet, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			if ip := net.ParseIP(entry); ip != nil {
				bits := 32
				if ip.To4() == nil {
					bits = 128
				}
				entry = fmt.Sprintf("%s/%d", entry, bits)
			}
		}
		_, network, err := net.ParseCIDR(entry)
		if err != nil {
			log.Printf("ignoring invalid TRUSTED_PROXIES entry %q: %v", entry, err)
			continue
		}
		networks = append(networks, network)
	}
	return networks
}

// clientIP returns the address the login limiter counts against. The
// X-Forwarded-For header is only consulted when the request actually came
// from a configured reverse proxy — otherwise any client could hand us a
// fresh fake address per attempt and never hit a limit at all.
func (app *App) clientIP(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		host = request.RemoteAddr
	}
	if !app.isTrustedProxy(net.ParseIP(host)) {
		return host
	}
	forwarded := request.Header.Get("X-Forwarded-For")
	if forwarded == "" {
		return host
	}
	// Left-most entry is the original client; the proxy appends, so only a
	// trusted proxy's own list is read here.
	original := strings.TrimSpace(strings.Split(forwarded, ",")[0])
	if net.ParseIP(original) == nil {
		return host
	}
	return original
}

func (app *App) isTrustedProxy(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, network := range app.trustedProxies {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// normalizeUsername collapses the spellings of one account onto a single
// limiter key, so "Tom", "tom " and "TOM" share a budget instead of getting
// five attempts each.
func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func userIPKey(username, clientIP string) string { return "user-ip:" + username + "|" + clientIP }
func userKey(username string) string             { return "user:" + username }

// countFailures returns how many failures under key fall inside the window.
// The caller holds loginLimitersMu.
func (app *App) countFailures(key string, now time.Time) int {
	limiter, exists := app.loginLimiters[key]
	if !exists {
		return 0
	}
	valid := limiter.attempts[:0]
	for _, t := range limiter.attempts {
		if now.Sub(t) < loginFailureWindow {
			valid = append(valid, t)
		}
	}
	limiter.attempts = valid
	return len(valid)
}

// loginDecision reports whether an attempt may proceed and how long it should
// be held back first. Only the source-scoped counter can refuse; the
// account-wide counter merely slows things down (see the const block).
func (app *App) loginDecision(username, clientIP string) (allowed bool, delay time.Duration) {
	app.loginLimitersMu.Lock()
	defer app.loginLimitersMu.Unlock()

	now := time.Now()
	app.sweepLoginLimiters(now)

	if app.countFailures(userIPKey(username, clientIP), now) >= loginFailuresPerUserAndIP {
		return false, 0
	}
	if app.countFailures(userKey(username), now) >= loginFailuresPerUser {
		return true, app.sprayDelay()
	}
	return true, 0
}

func (app *App) sprayDelay() time.Duration {
	if app.loginSprayDelay != 0 {
		return app.loginSprayDelay
	}
	return loginSprayDelay
}

// recordLoginFailure is called only after Emby actually rejected the
// credentials. Counting failures rather than requests means a user logging in
// correctly never accumulates anything, so normal use cannot drift into the
// throttle.
func (app *App) recordLoginFailure(username, clientIP string) {
	app.loginLimitersMu.Lock()
	defer app.loginLimitersMu.Unlock()

	now := time.Now()
	for _, key := range []string{userIPKey(username, clientIP), userKey(username)} {
		limiter, exists := app.loginLimiters[key]
		if !exists {
			limiter = &loginRateLimiter{}
			app.loginLimiters[key] = limiter
		}
		app.countFailures(key, now) // prunes the window in place
		limiter.attempts = append(limiter.attempts, now)
	}
}

// clearLoginFailures wipes the record for a source that just proved it knows
// the password, so one fat-fingered attempt does not linger for a minute.
func (app *App) clearLoginFailures(username, clientIP string) {
	app.loginLimitersMu.Lock()
	defer app.loginLimitersMu.Unlock()
	delete(app.loginLimiters, userIPKey(username, clientIP))
	delete(app.loginLimiters, userKey(username))
}

// sweepLoginLimiters drops entries whose failures have all aged out. Without
// it the map only ever grows: the keys contain an attacker-chosen username, so
// an unauthenticated caller could otherwise pin arbitrary memory by guessing
// against names that do not exist. Runs at most once per window — the map is
// small enough that a full pass at that rate is not worth optimising.
// The caller holds loginLimitersMu.
func (app *App) sweepLoginLimiters(now time.Time) {
	if now.Sub(app.loginLimitersSwept) < loginFailureWindow {
		return
	}
	app.loginLimitersSwept = now
	for key, limiter := range app.loginLimiters {
		kept := limiter.attempts[:0]
		for _, t := range limiter.attempts {
			if now.Sub(t) < loginFailureWindow {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(app.loginLimiters, key)
			continue
		}
		limiter.attempts = kept
	}
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

	username := normalizeUsername(input.Username)
	clientIP := app.clientIP(request)
	allowed, delay := app.loginDecision(username, clientIP)
	if !allowed {
		respondJSON(writer, http.StatusTooManyRequests, map[string]string{"error": "too many login attempts, try again later"})
		return
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-request.Context().Done():
			return
		}
	}
	identity, err := app.authenticator.Authenticate(request.Context(), emby.Credentials{Username: input.Username, Password: input.Password})
	if errors.Is(err, emby.ErrInvalidCredentials) {
		app.recordLoginFailure(username, clientIP)
		respondJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "Emby is unavailable"})
		return
	}
	app.clearLoginFailures(username, clientIP)
	// The first Emby account to ever log in successfully becomes the Emby
	// Insights admin, atomically — see appconfig.Store.ClaimAdminOwner. A
	// failure here is best-effort: it must never block a login, it just means
	// nobody becomes admin from this particular request.
	if app.appconfig != nil {
		_, _ = app.appconfig.ClaimAdminOwner(request.Context(), identity.UserID)
	}

	sessionID, err := app.sessions.Create(request.Context(), identity)
	if err != nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"error": "session storage is unavailable"})
		return
	}
	http.SetCookie(writer, &http.Cookie{Name: sessionCookieName, Value: sessionID, Path: "/", MaxAge: int(session.Duration.Seconds()), HttpOnly: true, Secure: app.cookieSecure, SameSite: http.SameSiteLaxMode})
	respondJSON(writer, http.StatusOK, app.identityProfile(request.Context(), identity))
}

func (app *App) me(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	respondJSON(writer, http.StatusOK, app.identityProfile(request.Context(), identity))
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
	statistics, err := cachedJSON(request.Context(), app, "stats:"+identity.UserID+":"+period, statsCacheTTL, func(ctx context.Context) (emby.PersonalWatchTime, error) {
		return app.statistics.PersonalWatchTime(ctx, identity.UserID, period)
	})
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
	rank, err := cachedJSON(request.Context(), app, "watchtimerank:"+identity.UserID, statsCacheTTL, func(ctx context.Context) (emby.WatchTimeRank, error) {
		return app.watchTimeRank.WatchTimeRank(ctx, identity.UserID)
	})
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
	devices, err := cachedJSON(request.Context(), app, "devicestats:"+identity.UserID+":"+period, statsCacheTTL, func(ctx context.Context) ([]emby.DeviceWatchTime, error) {
		return app.deviceStatistics.DeviceWatchTimes(ctx, identity.UserID, period)
	})
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
	hours, err := cachedJSON(request.Context(), app, "hourstats:"+identity.UserID+":"+period, statsCacheTTL, func(ctx context.Context) ([]emby.HourWatchTime, error) {
		return app.sessionStatistics.HourWatchTimes(ctx, identity.UserID, period)
	})
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
	weekdays, err := cachedJSON(request.Context(), app, "weekdaystats:"+identity.UserID+":"+period, statsCacheTTL, func(ctx context.Context) ([]emby.WeekdayWatchTime, error) {
		return app.sessionStatistics.WeekdayWatchTimes(ctx, identity.UserID, period)
	})
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
	type longestSessionResult struct {
		Session emby.LongestSession
		Found   bool
	}
	result, err := cachedJSON(request.Context(), app, "longestsession:"+identity.UserID+":"+period, statsCacheTTL, func(ctx context.Context) (longestSessionResult, error) {
		session, found, err := app.sessionStatistics.LongestSession(ctx, identity.UserID, period)
		return longestSessionResult{Session: session, Found: found}, err
	})
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "longest-session statistics are unavailable"})
		return
	}
	if !result.Found {
		respondJSON(writer, http.StatusOK, nil)
		return
	}
	respondJSON(writer, http.StatusOK, result.Session)
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
	type mostActiveDayResult struct {
		Day   emby.MostActiveDay
		Found bool
	}
	result, err := cachedJSON(request.Context(), app, "mostactiveday:"+identity.UserID+":"+period, statsCacheTTL, func(ctx context.Context) (mostActiveDayResult, error) {
		day, found, err := app.sessionStatistics.MostActiveDay(ctx, identity.UserID, period)
		return mostActiveDayResult{Day: day, Found: found}, err
	})
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "most-active-day statistics are unavailable"})
		return
	}
	if !result.Found {
		respondJSON(writer, http.StatusOK, nil)
		return
	}
	respondJSON(writer, http.StatusOK, result.Day)
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

// cachedJSON keeps upstream lookups off the request path for a short period.
// The upstream services remain the source of truth; a cache failure (miss,
// Redis down, corrupt payload) must never make the dashboard unavailable —
// it just falls through to read().
func cachedJSON[T any](ctx context.Context, app *App, key string, ttl time.Duration, read func(context.Context) (T, error)) (T, error) {
	if app.redis != nil {
		if value, err := app.redis.Get(ctx, key).Result(); err == nil {
			var cached T
			if json.Unmarshal([]byte(value), &cached) == nil {
				return cached, nil
			}
		}
	}
	result, err := read(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	if app.redis != nil {
		if value, err := json.Marshal(result); err == nil {
			_ = app.redis.Set(ctx, key, value, ttl).Err()
		}
	}
	return result, nil
}

func (app *App) cachedComingSoonItems(ctx context.Context, kind string, read func(context.Context) ([]comingsoon.Item, error)) ([]comingsoon.Item, error) {
	return cachedJSON(ctx, app, "comingsoon:"+kind, comingSoonCacheTTL, read)
}

func (app *App) myRequests(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := cachedJSON(request.Context(), app, "requests:"+identity.UserID, requestsCacheTTL, func(ctx context.Context) ([]seerr.Request, error) {
		return app.requests.Requests(ctx, identity.UserID)
	})
	if err != nil {
		log.Printf("requests unavailable for user %s: %v", identity.UserID, err)
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

func (app *App) discoverByProvider(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	providerID := request.URL.Query().Get("id")
	if providerID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	items, err := app.discover.DiscoverByProvider(request.Context(), providerID, app.live.discoverRegion())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "discover list is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

// discoverHandler builds a handler for one Seerr discover list. Discover
// data is not personal and shared by every user, so it is cached under a
// fixed key for discoverCacheTTL — the endpoint still requires a session
// like the rest of the API.
func (app *App) discoverHandler(cacheKey string, read func(context.Context, seerr.DiscoverReader) ([]seerr.DiscoverItem, error)) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := app.identityFromRequest(writer, request); !ok {
			return
		}
		items, err := cachedJSON(request.Context(), app, cacheKey, discoverCacheTTL, func(ctx context.Context) ([]seerr.DiscoverItem, error) {
			return read(ctx, app.discover)
		})
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
	// OMDb ratings are decoration, not core data — a lookup failure (not
	// configured, no IMDb id, OMDb down) must never turn an otherwise-fine
	// media-detail response into a 502, so this ignores the error.
	if omdbClient := app.live.omdbClient(); omdbClient != nil && detail.ImdbID != "" {
		if ratings, err := omdbClient.Ratings(request.Context(), detail.ImdbID); err == nil {
			detail.ImdbRating = ratings.ImdbRating
			detail.RottenTomatoesRating = ratings.RottenTomatoesRating
		}
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
	if app.activity != nil {
		if err := app.activity.RecordSeerrRequest(request.Context(), identity.UserID, input.MediaType, input.TmdbID); err != nil {
			log.Printf("recording seerr request for activity chart failed: %v", err)
		}
	}
	if app.redis != nil {
		_ = app.redis.Del(request.Context(), "requests:"+identity.UserID).Err()
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

	// Best-effort: cache the poster now, while entry.PosterURL is guaranteed
	// fresh (it was built moments ago when the detail screen loaded). A
	// failed fetch just leaves the image column untouched rather than
	// failing the whole save — see the "Top Bewertet" fix in CHANGELOG.
	if data, contentType, ok := app.fetchPosterBytes(request.Context(), entry.MediaSource, entry.PosterURL); ok {
		entry.PosterImageData = data
		entry.PosterImageContentType = contentType
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

// currentAdminOwner returns the Emby user id of whoever became the Emby
// Insights admin on their first successful login (see login and
// appconfig.Store.ClaimAdminOwner), or "" if nobody has logged in yet. A
// lookup failure is treated the same as "no admin yet" rather than a fatal
// error — it never blocks a request, it just denies admin access.
func (app *App) currentAdminOwner(ctx context.Context) string {
	if app.appconfig == nil {
		return ""
	}
	ownerID, err := app.appconfig.CurrentAdminOwner(ctx)
	if err != nil {
		return ""
	}
	return ownerID
}

// requireAdmin rejects the request unless the caller is the Emby Insights
// admin — the single Emby account that became admin by logging in first (see
// login). Chat and the Verwaltung admin endpoints are the only things this
// gates; there is no broader role system.
func (app *App) requireAdmin(writer http.ResponseWriter, request *http.Request, identity emby.Identity) bool {
	if ownerID := app.currentAdminOwner(request.Context()); ownerID == "" || identity.UserID != ownerID {
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
	if identity.UserID == app.currentAdminOwner(request.Context()) {
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
	if identity.UserID == app.currentAdminOwner(request.Context()) {
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
	if identity.UserID == app.currentAdminOwner(request.Context()) {
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

// adminAvatarForUser serves the admin's own avatar to any logged-in user —
// unlike adminUserAvatar (which lets the admin look up anyone), this is the
// one admin-related image every regular user is allowed to see, since it's
// their own chat partner's picture.
func (app *App) adminAvatarForUser(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	ownerID := app.currentAdminOwner(request.Context())
	if ownerID == "" {
		http.NotFound(writer, request)
		return
	}
	image, err := app.adminAvatars.UserPrimaryImageByID(request.Context(), ownerID)
	if errors.Is(err, emby.ErrPrimaryImageUnavailable) {
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "profile image is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", image.ContentType)
	writer.Header().Set("Cache-Control", "private, no-cache")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(image.Data)
}

func (app *App) adminMessageThreads(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
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
	if !app.requireAdmin(writer, request, identity) {
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
	if !app.requireAdmin(writer, request, identity) {
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
	if !app.requireAdmin(writer, request, identity) {
		return
	}
	users, err := app.directory.Users(request.Context())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "the user directory is unavailable"})
		return
	}
	ownerID := app.currentAdminOwner(request.Context())
	contacts := make([]map[string]string, 0, len(users))
	for _, user := range users {
		if user.ID == ownerID {
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
	if !app.requireAdmin(writer, request, identity) {
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
	writer.Header().Set("Cache-Control", "private, no-cache")
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
	if !app.requireAdmin(writer, request, identity) {
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
	ownerID := app.currentAdminOwner(request.Context())
	sent := 0
	for _, user := range users {
		if user.ID == ownerID {
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
	if !app.requireAdmin(writer, request, identity) {
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

func (app *App) adminDeleteThread(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
		return
	}
	userID := request.URL.Query().Get("userId")
	if userID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}
	if err := app.messages.DeleteThread(request.Context(), userID); err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "deleting the thread failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// adminLibraries lists Emby's libraries by name, for the Verwaltung UI's
// "Neu für dich" / "Gesehene Filme und Serien" library pickers — the operator
// never has to look up a library ID manually.
func (app *App) adminLibraries(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
		return
	}
	libraries, err := app.embyClient.Libraries(request.Context())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "Emby libraries are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(libraries))
}

// serviceSettingView is the browser-safe view of one optional integration:
// it never carries the plaintext API key, only whether one is stored and a
// masked preview, matching the product requirement that a saved key is never
// shown in full again.
type serviceSettingView struct {
	Enabled       bool   `json:"enabled"`
	BaseURL       string `json:"baseUrl,omitempty"`
	APIKeySet     bool   `json:"apiKeySet"`
	APIKeyPreview string `json:"apiKeyPreview,omitempty"`
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "••••"
	}
	return "••••" + key[len(key)-4:]
}

func viewOfService(setting appconfig.ServiceSetting) serviceSettingView {
	return serviceSettingView{
		Enabled:       setting.Enabled,
		BaseURL:       setting.BaseURL,
		APIKeySet:     setting.APIKey != "",
		APIKeyPreview: maskAPIKey(setting.APIKey),
	}
}

func validateServiceURL(baseURL string) error {
	if baseURL == "" {
		return nil
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https schemes allowed")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("URL has no hostname")
	}

	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return fmt.Errorf("localhost and loopback addresses not allowed")
	}

	// Private LAN ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16) are
	// deliberately allowed: Seerr/Radarr/Sonarr run on the operator's own
	// home network in the overwhelming majority of installs, and this app
	// itself typically runs there too — rejecting private IPs made it
	// impossible to save Verwaltung settings for that setup at all. What
	// actually needs blocking is loopback (the container reaching back into
	// itself) and link-local addresses, which include the 169.254.169.254
	// cloud metadata endpoint.
	ip := net.ParseIP(host)
	if ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return fmt.Errorf("loopback and link-local addresses not allowed: %s", host)
	}

	return nil
}

func (app *App) adminGetSettings(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
		return
	}
	settings, err := app.appconfig.Get(request.Context())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "settings are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, map[string]any{
		"newForYouLibraryIds": orEmpty(settings.NewForYouLibraryIDs),
		"watchedLibraryIds":   orEmpty(settings.WatchedLibraryIDs),
		"seerr":               viewOfService(settings.Seerr),
		"radarr":              viewOfService(settings.Radarr),
		"sonarr":              viewOfService(settings.Sonarr),
		"tmdb":                viewOfService(settings.TMDB),
		"omdb":                viewOfService(settings.OMDB),
		"comingSoonRegion":    settings.ComingSoonRegion,
		"comingSoonDaysAhead": settings.ComingSoonDaysAhead,
		"language":            uiLanguage(settings.Language),
	})
}

// adminPutSettings saves new Verwaltung settings and immediately hot-swaps
// the live Seerr/Radarr/Sonarr/TMDB clients (see App.applySettings) — the
// operator never has to restart the container after a change. An omitted or
// empty apiKey on any service means "keep the currently stored key".
func (app *App) adminPutSettings(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
		return
	}

	var input struct {
		NewForYouLibraryIDs []string `json:"newForYouLibraryIds"`
		WatchedLibraryIDs   []string `json:"watchedLibraryIds"`
		Seerr               struct {
			Enabled bool   `json:"enabled"`
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
		} `json:"seerr"`
		Radarr struct {
			Enabled bool   `json:"enabled"`
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
		} `json:"radarr"`
		Sonarr struct {
			Enabled bool   `json:"enabled"`
			BaseURL string `json:"baseUrl"`
			APIKey  string `json:"apiKey"`
		} `json:"sonarr"`
		TMDB struct {
			Enabled bool   `json:"enabled"`
			APIKey  string `json:"apiKey"`
		} `json:"tmdb"`
		OMDB struct {
			Enabled bool   `json:"enabled"`
			APIKey  string `json:"apiKey"`
		} `json:"omdb"`
		ComingSoonRegion    string `json:"comingSoonRegion"`
		ComingSoonDaysAhead int    `json:"comingSoonDaysAhead"`
		Language            string `json:"language"`
	}
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<16)
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	region := strings.TrimSpace(input.ComingSoonRegion)
	if len(region) != 2 {
		region = "DE"
	}
	daysAhead := input.ComingSoonDaysAhead
	if daysAhead <= 0 {
		daysAhead = 28
	}

	seerrURL := strings.TrimSpace(input.Seerr.BaseURL)
	radarrURL := strings.TrimSpace(input.Radarr.BaseURL)
	sonarrURL := strings.TrimSpace(input.Sonarr.BaseURL)

	if err := validateServiceURL(seerrURL); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid Seerr URL: %v", err)})
		return
	}
	if err := validateServiceURL(radarrURL); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid Radarr URL: %v", err)})
		return
	}
	if err := validateServiceURL(sonarrURL); err != nil {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid Sonarr URL: %v", err)})
		return
	}

	settings := appconfig.Settings{
		NewForYouLibraryIDs: input.NewForYouLibraryIDs,
		WatchedLibraryIDs:   input.WatchedLibraryIDs,
		Seerr:               appconfig.ServiceSetting{Enabled: input.Seerr.Enabled, BaseURL: seerrURL, APIKey: input.Seerr.APIKey},
		Radarr:              appconfig.ServiceSetting{Enabled: input.Radarr.Enabled, BaseURL: radarrURL, APIKey: input.Radarr.APIKey},
		Sonarr:              appconfig.ServiceSetting{Enabled: input.Sonarr.Enabled, BaseURL: sonarrURL, APIKey: input.Sonarr.APIKey},
		TMDB:                appconfig.ServiceSetting{Enabled: input.TMDB.Enabled, APIKey: input.TMDB.APIKey},
		OMDB:                appconfig.ServiceSetting{Enabled: input.OMDB.Enabled, APIKey: input.OMDB.APIKey},
		ComingSoonRegion:    region,
		ComingSoonDaysAhead: daysAhead,
		Language:            uiLanguage(input.Language),
	}

	if err := app.applySettings(request.Context(), settings); err != nil {
		log.Printf("saving Verwaltung settings failed: %v", err)
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "saving settings failed"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// adminDebugLive reports the currently live, in-memory integration state —
// as opposed to GET /api/admin/settings, which reports what's persisted in
// the database. The two can disagree if applySettings failed to rebuild the
// live clients after a save; this endpoint exists to tell that case apart
// from a genuinely misconfigured or unreachable service.
func (app *App) adminDebugLive(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
		return
	}
	seerrClient, comingSoonClient, region := app.live.current()
	respondJSON(writer, http.StatusOK, map[string]any{
		"seerrConfigured":      seerrClient != nil,
		"comingSoonConfigured": comingSoonClient != nil,
		"omdbConfigured":       app.live.omdbClient() != nil,
		"comingSoonRegion":     region,
		"newForYouLibraryIds":  orEmpty(app.live.newForYouLibraries()),
		"watchedLibraryIds":    orEmpty(app.live.watchedLibraries()),
	})
}

const activityChartDays = 7

// adminActivity backs the Verwaltung activity chart: how many Seerr
// requests Emby Insights actually triggered, and how many distinct users
// were active, per day, for the last week.
func (app *App) adminActivity(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	if !app.requireAdmin(writer, request, identity) {
		return
	}
	if app.activity == nil {
		respondJSON(writer, http.StatusOK, []store.DailyActivity{})
		return
	}
	days, err := app.activity.WeeklyActivity(request.Context(), activityChartDays)
	if err != nil {
		respondJSON(writer, http.StatusInternalServerError, map[string]string{"error": "activity data is unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(days))
}

// validEmbyItemID matches the shapes Emby actually uses for item IDs — hex
// GUIDs (with or without dashes) and numeric IDs. Anything else is rejected
// before it can reach SetFavorite, which interpolates the ID into a URL sent
// with the admin API key.
var validEmbyItemID = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// validImageTag matches Emby's image tags — usually a GUID plus a tick
// count joined with an underscore (e.g. "320bb58c..._639205781665227668"),
// but items with multiple image versions get longer, multi-segment tags —
// which are interpolated into a URL sent to Emby with the admin API key.
var validImageTag = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)

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

// setPlayedHandler returns a handler that marks (or, if favorite is false,
// unmarks) an Emby item as fully watched. It follows the same shape as
// setFavoriteHandler since both are thin, validated passthroughs to an Emby
// per-user item-state endpoint.
func (app *App) setPlayedHandler(played bool) http.HandlerFunc {
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
		if err := app.playedItems.SetPlayed(request.Context(), identity.UserID, itemID, played); err != nil {
			respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "updating the Emby played state failed"})
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
	items, err := app.newForYou.NewForYou(request.Context(), identity.UserID, app.live.newForYouLibraries())
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

func (app *App) topRatedHandler(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	items, err := app.tracking.TopRatings(request.Context(), topRatedLimit)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "top rated titles are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) watchedMovies(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.watched.WatchedMovies(request.Context(), identity.UserID, app.live.watchedLibraries())
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
	items, err := app.watched.WatchedSeries(request.Context(), identity.UserID, app.live.watchedLibraries())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "watched series are unavailable"})
		return
	}
	respondJSON(writer, http.StatusOK, orEmpty(items))
}

func (app *App) seriesInProgressHandler(writer http.ResponseWriter, request *http.Request) {
	identity, ok := app.identityFromRequest(writer, request)
	if !ok {
		return
	}
	items, err := app.seriesInProgress.SeriesInProgress(request.Context(), identity.UserID, app.live.watchedLibraries())
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "series in progress are unavailable"})
		return
	}
	// A failure to load dismissals must not hide the whole row — it just
	// means a previously-dismissed series briefly reappears.
	if hidden, err := app.tracking.HiddenInProgressIDs(request.Context(), identity.UserID); err == nil && len(hidden) > 0 {
		visible := make([]emby.SeriesProgress, 0, len(items))
		for _, item := range items {
			if !hidden[item.ID] {
				visible = append(visible, item)
			}
		}
		items = visible
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
	items, err := app.completed.CompletedMovies(request.Context(), identity.UserID, app.live.watchedLibraries(), from, to)
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
	items, err := app.completed.CompletedSeries(request.Context(), identity.UserID, app.live.watchedLibraries(), from, to)
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
	app.recordActivity(identity.UserID)
	return identity, true
}

// recordActivity marks a user active for today's Verwaltung activity chart.
// It is fire-and-forget and must never add latency or a failure mode to the
// request that triggered it — every authenticated endpoint runs through
// identityFromRequest, so this fires on nearly every API call. A Redis
// SETNX gates the (much rarer) Postgres write to once per user per day;
// if Redis is unavailable, activity for that request is simply not
// recorded rather than falling back to writing Postgres on every call.
func (app *App) recordActivity(embyUserID string) {
	if app.redis == nil || app.activity == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		today := time.Now().UTC()
		key := "activity:" + today.Format("2006-01-02") + ":" + embyUserID
		first, err := app.redis.SetNX(ctx, key, "1", 26*time.Hour).Result()
		if err != nil || !first {
			return
		}
		if err := app.activity.RecordActive(ctx, embyUserID, today); err != nil {
			log.Printf("recording daily activity failed: %v", err)
		}
	}()
}

// uiLanguage clamps a stored or submitted value to a language the frontend
// actually ships a dictionary for. An unknown value is silently normalised
// to German rather than rejected, so a hand-edited database row or an older
// client can never lock the admin out of the settings page.
func uiLanguage(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en":
		return "en"
	default:
		return "de"
	}
}

// uiLanguageHandler serves the global UI language to unauthenticated clients.
// Any failure degrades to German instead of erroring: the login screen must
// render even when the database is briefly unavailable.
func (app *App) uiLanguageHandler(writer http.ResponseWriter, request *http.Request) {
	language := "de"
	if app.appconfig != nil {
		if settings, err := app.appconfig.Get(request.Context()); err == nil {
			language = uiLanguage(settings.Language)
		}
	}
	respondJSON(writer, http.StatusOK, map[string]string{"language": language})
}

func (app *App) identityProfile(ctx context.Context, identity emby.Identity) map[string]any {
	var settings appconfig.Settings
	if app.appconfig != nil {
		if got, err := app.appconfig.Get(ctx); err == nil {
			settings = got
		}
	}
	ownerID := app.currentAdminOwner(ctx)
	return map[string]any{
		"id":       identity.UserID,
		"name":     identity.DisplayName,
		"isAdmin":  ownerID != "" && identity.UserID == ownerID,
		"language": uiLanguage(settings.Language),
		"features": map[string]bool{
			"requests":    settings.Seerr.Enabled,
			"movieDates":  settings.Radarr.Enabled,
			"seriesDates": settings.Sonarr.Enabled,
			"upcoming":    settings.Radarr.Enabled || settings.Sonarr.Enabled,
			// Statistik always shows: unlike Seerr/Radarr/Sonarr/TMDB, the Emby
			// Insights connector/Playback Reporting isn't something the admin
			// toggles in Verwaltung, and there is no reliable signal to detect
			// it's missing without a live Emby call on every /api/me request.
			// media_tracking (ratings/watchlist) was tried and rejected as a
			// proxy: it stays empty until a user manually rates/bookmarks
			// something, which hid Statistik even with playback data present.
			"statistics": true,
		},
	}
}

func respondJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	// Every JSON response here is per-user or reflects live upstream state
	// (Seerr request/season status, watch stats, ...) — none of it may be
	// reused from a browser or intermediate cache for a later request.
	writer.Header().Set("Cache-Control", "no-store")
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
	writer.Header().Set("Cache-Control", "private, no-cache")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(image.Data)
}

// itemImage proxies Emby posters/backdrops through this server instead of
// linking the browser straight to Emby's own address (see emby.ImageURL) —
// otherwise anyone reaching the dashboard through a reverse proxy from
// outside Emby's own network gets broken images. Responses are cached in
// Redis under a key that includes Emby's image tag, so a repeat view never
// has to round-trip to Emby again.
func (app *App) itemImage(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	itemID := request.URL.Query().Get("itemId")
	if itemID == "" || !validEmbyItemID.MatchString(itemID) {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "itemId is required"})
		return
	}
	imageType := request.URL.Query().Get("type")
	if imageType != "Primary" && imageType != "Backdrop" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "type must be Primary or Backdrop"})
		return
	}
	tag := request.URL.Query().Get("tag")
	if !validImageTag.MatchString(tag) {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "tag is invalid"})
		return
	}
	maxWidth, err := strconv.Atoi(request.URL.Query().Get("maxWidth"))
	if err != nil || maxWidth < 1 || maxWidth > 2000 {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "maxWidth is invalid"})
		return
	}

	cacheKey := fmt.Sprintf("image:%s:%s:%s:%d", itemID, imageType, tag, maxWidth)
	image, err := cachedJSON(request.Context(), app, cacheKey, imageCacheTTL, func(ctx context.Context) (emby.UserImage, error) {
		return app.embyClient.ItemImage(ctx, itemID, imageType, tag, maxWidth)
	})
	if errors.Is(err, emby.ErrItemImageUnavailable) {
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "image is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", image.ContentType)
	writer.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(image.Data)
}

// trackingPosterImage serves the poster bytes cached alongside a rating
// (migration 009) — the "Top Bewertet" home row and a user's own ratings/
// watchlist all link here instead of a raw Emby/TMDB URL, so a title's
// poster survives Emby regenerating its artwork tag or removing the item
// entirely after it was rated.
func (app *App) trackingPosterImage(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	mediaSource := request.URL.Query().Get("source")
	mediaID := request.URL.Query().Get("id")
	if mediaSource == "" || mediaID == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "source and id are required"})
		return
	}
	data, _, found, err := app.tracking.PosterImage(request.Context(), mediaSource, mediaID)
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "poster is unavailable"})
		return
	}
	if !found {
		http.NotFound(writer, request)
		return
	}
	// The stored content type is deliberately discarded and re-derived from
	// the bytes. Rows written before poster fetching was hardened could hold
	// a non-image body with an attacker-chosen content type; re-sniffing on
	// every read means those simply stop being served instead of needing a
	// migration to find them.
	contentType, ok := detectImageContentType(data)
	if !ok {
		http.NotFound(writer, request)
		return
	}
	writer.Header().Set("Content-Type", contentType)
	writer.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(data)
}

// fetchPosterBytes resolves a poster URL to actual image bytes so it can be
// persisted (see upsertTracking and BackfillPosterImages). Emby-sourced URLs
// are our own /api/images proxy links (see emby.ImageURL) — those go
// straight to Emby with the admin API key, bypassing the proxy's Redis cache
// since this only ever runs once per rating.
//
// Anything else is a Seerr/TMDB URL that arrived in a request body, so it is
// attacker-controlled and is fetched through app.imageFetchClient: public
// addresses only, no redirects, hard timeout. Whatever comes back — from
// either branch — has to sniff as a real image before it is stored, so the
// content type in the database is one we determined, never one a remote
// server claimed.
func (app *App) fetchPosterBytes(ctx context.Context, mediaSource, posterURL string) ([]byte, string, bool) {
	if posterURL == "" {
		return nil, "", false
	}
	if mediaSource == "emby" {
		itemID, tag, maxWidth, ok := parseImageProxyURL(posterURL)
		if !ok {
			return nil, "", false
		}
		image, err := app.embyClient.ItemImage(ctx, itemID, "Primary", tag, maxWidth)
		if err != nil {
			return nil, "", false
		}
		if len(image.Data) > maxPosterBytes {
			return nil, "", false
		}
		contentType, ok := detectImageContentType(image.Data)
		if !ok {
			return nil, "", false
		}
		return image.Data, contentType, true
	}

	parsed, err := url.Parse(posterURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, "", false
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, posterURL, nil)
	if err != nil {
		return nil, "", false
	}
	response, err := app.imageFetchClient.Do(request)
	if err != nil {
		return nil, "", false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", false
	}
	return readImageBody(response.Body)
}

// parseImageProxyURL extracts the itemId/tag/maxWidth query parameters back
// out of a URL built by emby.ImageURL.
func parseImageProxyURL(raw string) (itemID, tag string, maxWidth int, ok bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", "", 0, false
	}
	query := parsed.Query()
	itemID = query.Get("itemId")
	tag = query.Get("tag")
	width, err := strconv.Atoi(query.Get("maxWidth"))
	if itemID == "" || tag == "" || err != nil {
		return "", "", 0, false
	}
	return itemID, tag, width, true
}

// artworkImage serves posters that live on a public artwork CDN through our
// own origin. Two reasons it exists rather than letting the browser fetch the
// CDN directly: the page's Content-Security-Policy can stay at `img-src
// 'self'`, and the CDN never learns which titles a given user is browsing.
//
// The URL is only ever produced by artwork.ProxyURL, but it arrives back as a
// query parameter, so it is treated as untrusted: the host is checked against
// the allow list again here, and the fetch goes through the same locked-down
// client as remote poster fetching — public addresses only, no redirects, hard
// timeout, and the content type is derived from the bytes.
func (app *App) artworkImage(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	raw := request.URL.Query().Get("u")
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || !artwork.AllowedHost(parsed.Host) {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "u must be an artwork URL on a supported host"})
		return
	}

	cacheKey := "artwork:" + parsed.String()
	image, err := cachedJSON(request.Context(), app, cacheKey, imageCacheTTL, func(ctx context.Context) (emby.UserImage, error) {
		fetchRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return emby.UserImage{}, err
		}
		response, err := app.imageFetchClient.Do(fetchRequest)
		if err != nil {
			return emby.UserImage{}, fmt.Errorf("fetch artwork: %w", err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return emby.UserImage{}, fmt.Errorf("%w: artwork CDN returned %s", emby.ErrItemImageUnavailable, response.Status)
		}
		data, contentType, ok := readImageBody(response.Body)
		if !ok {
			return emby.UserImage{}, fmt.Errorf("%w: artwork is not a usable image", emby.ErrItemImageUnavailable)
		}
		return emby.UserImage{ContentType: contentType, Data: data}, nil
	})
	if errors.Is(err, emby.ErrItemImageUnavailable) {
		http.NotFound(writer, request)
		return
	}
	if err != nil {
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "artwork is unavailable"})
		return
	}
	writer.Header().Set("Content-Type", image.ContentType)
	writer.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(image.Data)
}

// comingSoonMediaDetailHandler serves the detail screen for a Radarr or Sonarr
// calendar entry, built from what those services already returned when the
// list was assembled. It exists because Seerr is optional: without it the
// "Demnächst" and "Im Kino" tiles had no detail screen at all, and for series
// not even a usable id, since translating Sonarr's TVDB id into a TMDB one is
// itself a TMDB call.
//
// The response deliberately mirrors the shape the Seerr detail returns, so the
// frontend renders both with the same component. Cast, crew and seasons stay
// empty — Radarr and Sonarr do not carry them.
func (app *App) comingSoonMediaDetailHandler(writer http.ResponseWriter, request *http.Request) {
	if _, ok := app.identityFromRequest(writer, request); !ok {
		return
	}
	source := request.URL.Query().Get("source")
	id := request.URL.Query().Get("id")
	if (source != comingsoon.SourceRadarr && source != comingsoon.SourceSonarr) || id == "" {
		respondJSON(writer, http.StatusBadRequest, map[string]string{"error": "source must be radarr or sonarr, and id is required"})
		return
	}
	if app.comingSoon == nil {
		respondJSON(writer, http.StatusNotFound, map[string]string{"error": "no release calendar is configured"})
		return
	}
	item, found, err := app.comingSoon.Detail(request.Context(), source, id)
	if err != nil {
		log.Printf("coming-soon detail unavailable for %s/%s: %v", source, id, err)
		respondJSON(writer, http.StatusBadGateway, map[string]string{"error": "media detail is unavailable"})
		return
	}
	if !found {
		http.NotFound(writer, request)
		return
	}

	detail := seerr.MediaDetail{
		ID:              item.ID,
		Title:           item.Title,
		Overview:        item.Overview,
		PosterURL:       item.PosterURL,
		BackdropURL:     item.BackdropURL,
		Genres:          orEmpty(item.Genres),
		CommunityRating: item.Rating,
		Year:            item.Year,
		RuntimeMinutes:  item.RuntimeMinutes,
		Cast:            []seerr.Person{},
		Crew:            []seerr.Person{},
		Seasons:         []seerr.RequestableSeason{},
		Status:          item.Status,
		ReleaseDate:     item.AvailabilityDate,
		OfficialRating:  item.OfficialRating,
	}
	if item.Studio != "" {
		detail.Studios = []string{item.Studio}
	}
	respondJSON(writer, http.StatusOK, detail)
}
