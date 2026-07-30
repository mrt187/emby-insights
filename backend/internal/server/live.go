package server

import (
	"context"
	"sync"
	"time"

	"github.com/mrt187/EmbyInsights/internal/comingsoon"
	"github.com/mrt187/EmbyInsights/internal/omdb"
	"github.com/mrt187/EmbyInsights/internal/seerr"
)

// liveConfig holds everything the setup wizard's Verwaltung UI can change at
// runtime: the Seerr and Radarr/Sonarr/TMDB clients, the discover region, and
// the Emby library selections. App's interface fields (seerr.*Reader,
// comingsoon.Reader) point at the thin wrappers below instead of a client
// directly, so PUT /api/admin/settings can swap everything in place via
// set() without a container restart or any handler change.
type liveConfig struct {
	mu                  sync.RWMutex
	seerr               *seerr.Client
	comingSoon          *comingsoon.Client
	omdb                *omdb.Client
	region              string
	newForYouLibraryIDs []string
	watchedLibraryIDs   []string
}

func (live *liveConfig) set(seerrClient *seerr.Client, comingSoonClient *comingsoon.Client, omdbClient *omdb.Client, region string, newForYouLibraryIDs, watchedLibraryIDs []string) {
	live.mu.Lock()
	defer live.mu.Unlock()
	live.seerr = seerrClient
	live.comingSoon = comingSoonClient
	live.omdb = omdbClient
	live.region = region
	live.newForYouLibraryIDs = newForYouLibraryIDs
	live.watchedLibraryIDs = watchedLibraryIDs
}

// current, and every accessor below, is nil-safe: a bare &App{} (as most
// handler tests construct) never sets live, and nil seerr/comingsoon clients
// are already the established "not configured" state those packages use
// throughout.
func (live *liveConfig) current() (*seerr.Client, *comingsoon.Client, string) {
	if live == nil {
		return nil, nil, ""
	}
	live.mu.RLock()
	defer live.mu.RUnlock()
	return live.seerr, live.comingSoon, live.region
}

func (live *liveConfig) omdbClient() *omdb.Client {
	if live == nil {
		return nil
	}
	live.mu.RLock()
	defer live.mu.RUnlock()
	return live.omdb
}

func (live *liveConfig) discoverRegion() string {
	_, _, region := live.current()
	return region
}

func (live *liveConfig) newForYouLibraries() []string {
	if live == nil {
		return nil
	}
	live.mu.RLock()
	defer live.mu.RUnlock()
	return live.newForYouLibraryIDs
}

func (live *liveConfig) watchedLibraries() []string {
	if live == nil {
		return nil
	}
	live.mu.RLock()
	defer live.mu.RUnlock()
	return live.watchedLibraryIDs
}

// liveSeerr implements every seerr reader/writer interface App depends on by
// delegating to whichever *seerr.Client is currently live. seerr.Client
// methods are already nil-safe, so this works even before Seerr is
// configured for the first time.
type liveSeerr struct{ live *liveConfig }

func (wrapper liveSeerr) Requests(ctx context.Context, embyUserID string) ([]seerr.Request, error) {
	client, _, _ := wrapper.live.current()
	return client.Requests(ctx, embyUserID)
}

func (wrapper liveSeerr) AvailableRequests(ctx context.Context, embyUserID string, since time.Time) ([]seerr.Request, error) {
	client, _, _ := wrapper.live.current()
	return client.AvailableRequests(ctx, embyUserID, since)
}

func (wrapper liveSeerr) RequestStats(ctx context.Context, embyUserID string) (seerr.RequestStats, error) {
	client, _, _ := wrapper.live.current()
	return client.RequestStats(ctx, embyUserID)
}

func (wrapper liveSeerr) Trending(ctx context.Context) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.Trending(ctx)
}

func (wrapper liveSeerr) PopularMovies(ctx context.Context) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.PopularMovies(ctx)
}

func (wrapper liveSeerr) PopularSeries(ctx context.Context) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.PopularSeries(ctx)
}

func (wrapper liveSeerr) UpcomingMovies(ctx context.Context) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.UpcomingMovies(ctx)
}

func (wrapper liveSeerr) UpcomingSeries(ctx context.Context) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.UpcomingSeries(ctx)
}

func (wrapper liveSeerr) Search(ctx context.Context, query string) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.Search(ctx, query)
}

func (wrapper liveSeerr) DiscoverByProvider(ctx context.Context, providerID, region string) ([]seerr.DiscoverItem, error) {
	client, _, _ := wrapper.live.current()
	return client.DiscoverByProvider(ctx, providerID, region)
}

func (wrapper liveSeerr) MediaDetail(ctx context.Context, mediaType string, tmdbID int) (seerr.MediaDetail, error) {
	client, _, _ := wrapper.live.current()
	return client.MediaDetail(ctx, mediaType, tmdbID)
}

func (wrapper liveSeerr) CreateRequest(ctx context.Context, embyUserID, mediaType string, tmdbID int, seasons []int) error {
	client, _, _ := wrapper.live.current()
	return client.CreateRequest(ctx, embyUserID, mediaType, tmdbID, seasons)
}

// liveComingSoon implements comingsoon.Reader by delegating to whichever
// *comingsoon.Client is currently live. comingsoon.Client methods are
// already nil-safe and independently gated per Radarr/Sonarr.
type liveComingSoon struct{ live *liveConfig }

func (wrapper liveComingSoon) Upcoming(ctx context.Context) ([]comingsoon.Item, error) {
	_, client, _ := wrapper.live.current()
	return client.Upcoming(ctx)
}

func (wrapper liveComingSoon) InCinemas(ctx context.Context) ([]comingsoon.Item, error) {
	_, client, _ := wrapper.live.current()
	return client.InCinemas(ctx)
}
