// Package comingsoon reads the release calendars directly from Radarr, Sonarr
// and TMDB. It deliberately does not create Emby library items.
package comingsoon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mrt187/EmbyInsights/internal/artwork"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const posterBaseURL = "https://image.tmdb.org/t/p/w500"
const cinemaWindowDays = 30

type Item struct {
	ID     string `json:"id"`
	TmdbID string `json:"tmdbId"`
	// Source names the service this entry came from, and DetailID is the id
	// that service knows it by — Radarr hands us a TMDB id, Sonarr only a
	// TVDB one. Together they identify an entry without TMDB being
	// configured, which is what TmdbID alone cannot do: translating a TVDB id
	// into a TMDB one is itself a TMDB call, so series used to end up with an
	// empty id and a detail screen that could never load.
	Source           string `json:"source"`
	DetailID         string `json:"detailId"`
	Title            string `json:"title"`
	PosterURL        string `json:"posterUrl"`
	MediaType        string `json:"mediaType"`
	AvailabilityDate string `json:"availabilityDate"`
	CinemaStartDate  string `json:"cinemaStartDate,omitempty"`
	CinemaEndDate    string `json:"cinemaEndDate,omitempty"`
	SeasonNumber     int    `json:"seasonNumber,omitempty"`
	EpisodeNumber    int    `json:"episodeNumber,omitempty"`
	EpisodeTitle     string `json:"episodeTitle,omitempty"`

	// The fields below come from the same calendar response the list is built
	// from — Radarr and Sonarr return their full movie/series resource there,
	// so a detail screen costs no additional request.
	Overview       string   `json:"overview,omitempty"`
	Genres         []string `json:"genres,omitempty"`
	RuntimeMinutes int      `json:"runtimeMinutes,omitempty"`
	Year           int      `json:"year,omitempty"`
	OfficialRating string   `json:"officialRating,omitempty"`
	Rating         float64  `json:"rating,omitempty"`
	Studio         string   `json:"studio,omitempty"`
	Status         string   `json:"status,omitempty"`
	BackdropURL    string   `json:"backdropUrl,omitempty"`
}

// Sources the detail lookup accepts.
const (
	SourceRadarr = "radarr"
	SourceSonarr = "sonarr"
)

type Reader interface {
	Upcoming(context.Context) ([]Item, error)
	InCinemas(context.Context) ([]Item, error)
	Detail(ctx context.Context, source, id string) (Item, bool, error)
}

type Client struct {
	radarrURL, radarrKey string
	sonarrURL, sonarrKey string
	tmdbKey, region      string
	daysAhead            int
	httpClient           *http.Client
}

func NewClient(radarrURL, radarrKey, sonarrURL, sonarrKey, tmdbKey, region string, daysAhead int) *Client {
	if radarrURL == "" && sonarrURL == "" {
		return nil
	}
	if daysAhead <= 0 {
		daysAhead = 28
	}
	region = strings.ToUpper(strings.TrimSpace(region))
	if len(region) != 2 {
		region = "DE"
	}
	return &Client{
		radarrURL: strings.TrimRight(radarrURL, "/"), radarrKey: radarrKey,
		sonarrURL: strings.TrimRight(sonarrURL, "/"), sonarrKey: sonarrKey,
		tmdbKey: tmdbKey, region: region, daysAhead: daysAhead,
		httpClient: &http.Client{Timeout: 12 * time.Second},
	}
}

func (client *Client) Upcoming(ctx context.Context) ([]Item, error) {
	if client == nil {
		return nil, nil
	}
	now := time.Now().UTC()
	horizon := now.AddDate(0, 0, client.daysAhead)
	var items []Item
	movies, err := client.movies(ctx)
	if err != nil {
		return nil, err
	}
	for _, movie := range movies {
		dates, err := client.movieDates(ctx, movie.TmdbID, movie)
		if err != nil {
			return nil, err
		}
		if dates.digital.After(now) && !dates.digital.After(horizon) {
			items = append(items, movie.item(dates.digital, dates.digital, "movie"))
		}
	}
	episodes, err := client.episodes(ctx)
	if err != nil {
		return nil, err
	}
	for _, episode := range episodes {
		if episode.AirDate.After(now) && !episode.AirDate.After(horizon) {
			tmdbID, err := client.findTMDBTV(ctx, episode.TvdbID)
			if err != nil {
				return nil, err
			}
			// ID falls back to the Sonarr/TVDB id: without TMDB configured
			// findTMDBTV returns nothing, and an entry with an empty id used
			// to be appended anyway — a tile that could never open.
			id := tmdbID
			if id == "" {
				id = fmt.Sprintf("%d", episode.TvdbID)
			}
			items = append(items, episode.detail(Item{ID: id, TmdbID: tmdbID, Title: episode.Title, PosterURL: episode.PosterURL, MediaType: "tv", AvailabilityDate: episode.AirDate.Format(time.RFC3339), SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, EpisodeTitle: episode.EpisodeTitle}))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AvailabilityDate < items[j].AvailabilityDate })
	return items, nil
}

func (client *Client) InCinemas(ctx context.Context) ([]Item, error) {
	if client == nil || client.radarrURL == "" || client.radarrKey == "" {
		return nil, nil
	}
	now := time.Now().UTC()
	movies, err := client.movies(ctx)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, movie := range movies {
		dates, err := client.movieDates(ctx, movie.TmdbID, movie)
		if err != nil {
			return nil, err
		}
		// Cinema has a 30-day preview window. Films that already opened remain
		// visible for their full cinema run, until their digital release
		// arrives — and many films in cinemas right now don't have a digital
		// date announced yet, so a zero digital date must count as "still
		// running", not as "already available", or every such film silently
		// disappears from this row.
		windowEnd := now.AddDate(0, 0, cinemaWindowDays)
		stillRunning := dates.digital.IsZero() || dates.digital.After(now)
		if !dates.cinema.IsZero() && !dates.cinema.After(windowEnd) && stillRunning {
			items = append(items, movie.item(dates.cinema, dates.digital, "movie"))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CinemaStartDate < items[j].CinemaStartDate })
	return items, nil
}

type movie struct {
	Title, PosterURL, BackdropURL string
	TmdbID                        int
	Cinema, Digital               time.Time
	Overview, Certification       string
	Studio, Status                string
	Genres                        []string
	Runtime, Year                 int
	Rating                        float64
}

// detail copies the fields both calendars carry into the item, so Item stays
// the single shape the API returns for a list entry and for a detail lookup.
func (movie movie) detail(item Item) Item {
	item.Source = SourceRadarr
	item.DetailID = fmt.Sprintf("%d", movie.TmdbID)
	item.Overview = movie.Overview
	item.Genres = movie.Genres
	item.RuntimeMinutes = movie.Runtime
	item.Year = movie.Year
	item.OfficialRating = movie.Certification
	item.Rating = movie.Rating
	item.Studio = movie.Studio
	item.Status = movie.Status
	item.BackdropURL = movie.BackdropURL
	return item
}

func (movie movie) item(available, cinemaEnd time.Time, mediaType string) Item {
	id := fmt.Sprintf("%d", movie.TmdbID)
	var cinemaEndFormatted string
	if !cinemaEnd.IsZero() {
		cinemaEndFormatted = cinemaEnd.Format(time.RFC3339)
	}
	return movie.detail(Item{ID: id, TmdbID: id, Title: movie.Title, PosterURL: movie.PosterURL, MediaType: mediaType, AvailabilityDate: available.Format(time.RFC3339), CinemaStartDate: available.Format(time.RFC3339), CinemaEndDate: cinemaEndFormatted})
}

type episode struct {
	Title, PosterURL, BackdropURL       string
	TvdbID, SeasonNumber, EpisodeNumber int
	EpisodeTitle                        string
	AirDate                             time.Time
	Overview, Certification             string
	Studio, Status                      string
	Genres                              []string
	Runtime, Year                       int
	Rating                              float64
}

func (episode episode) detail(item Item) Item {
	item.Source = SourceSonarr
	item.DetailID = fmt.Sprintf("%d", episode.TvdbID)
	item.Overview = episode.Overview
	item.Genres = episode.Genres
	item.RuntimeMinutes = episode.Runtime
	item.Year = episode.Year
	item.OfficialRating = episode.Certification
	item.Rating = episode.Rating
	item.Studio = episode.Studio
	item.Status = episode.Status
	item.BackdropURL = episode.BackdropURL
	return item
}

type releaseDates struct{ cinema, digital time.Time }

func (client *Client) movies(ctx context.Context) ([]movie, error) {
	if client.radarrURL == "" || client.radarrKey == "" {
		return nil, nil
	}
	// InCinemas keeps a film visible for cinemaWindowDays after its cinema
	// release, but Radarr's own calendar only returns entries whose release
	// dates fall inside [start, end] — a film that opened, say, 10 days ago
	// with no digital date yet would already have fallen out of that window
	// and silently vanish from "Im Kino" if start only reached back 1 day,
	// no matter how generous the filtering further down is.
	start := time.Now().UTC().AddDate(0, 0, -cinemaWindowDays).Format("2006-01-02")
	end := time.Now().UTC().AddDate(0, 0, client.daysAhead+120).Format("2006-01-02")
	path := client.radarrURL + "/api/v3/calendar?start=" + start + "&end=" + end + "&unmonitored=false"
	var result []struct {
		Title          string    `json:"title"`
		TmdbID         int       `json:"tmdbId"`
		InCinemas      time.Time `json:"inCinemas"`
		DigitalRelease time.Time `json:"digitalRelease"`
		Overview       string    `json:"overview"`
		Runtime        int       `json:"runtime"`
		Year           int       `json:"year"`
		Certification  string    `json:"certification"`
		Studio         string    `json:"studio"`
		Status         string    `json:"status"`
		Genres         []string  `json:"genres"`
		Ratings        struct {
			Tmdb struct {
				Value float64 `json:"value"`
			} `json:"tmdb"`
			Imdb struct {
				Value float64 `json:"value"`
			} `json:"imdb"`
		} `json:"ratings"`
		Images []struct {
			CoverType string `json:"coverType"`
			RemoteURL string `json:"remoteUrl"`
		} `json:"images"`
	}
	if err := client.get(ctx, path, client.radarrKey, &result); err != nil {
		return nil, fmt.Errorf("read Radarr calendar: %w", err)
	}
	items := make([]movie, 0, len(result))
	for _, entry := range result {
		if entry.TmdbID != 0 {
			rating := entry.Ratings.Tmdb.Value
			if rating == 0 {
				rating = entry.Ratings.Imdb.Value
			}
			items = append(items, movie{
				Title: entry.Title, TmdbID: entry.TmdbID, Cinema: entry.InCinemas, Digital: entry.DigitalRelease,
				PosterURL: poster(entry.Images), BackdropURL: image(entry.Images, "fanart"),
				Overview: entry.Overview, Runtime: entry.Runtime, Year: entry.Year,
				Certification: entry.Certification, Studio: entry.Studio, Status: entry.Status,
				Genres: entry.Genres, Rating: rating,
			})
		}
	}
	return items, nil
}

func (client *Client) episodes(ctx context.Context) ([]episode, error) {
	if client.sonarrURL == "" || client.sonarrKey == "" {
		return nil, nil
	}
	start := time.Now().UTC().Format("2006-01-02")
	end := time.Now().UTC().AddDate(0, 0, client.daysAhead).Format("2006-01-02")
	path := client.sonarrURL + "/api/v3/calendar?start=" + start + "&end=" + end + "&includeSeries=true&unmonitored=false"
	var result []struct {
		Title         string    `json:"title"`
		SeasonNumber  int       `json:"seasonNumber"`
		EpisodeNumber int       `json:"episodeNumber"`
		AirDate       time.Time `json:"airDateUtc"`
		Series        struct {
			Title         string   `json:"title"`
			TvdbID        int      `json:"tvdbId"`
			Overview      string   `json:"overview"`
			Year          int      `json:"year"`
			Runtime       int      `json:"runtime"`
			Network       string   `json:"network"`
			Status        string   `json:"status"`
			Certification string   `json:"certification"`
			Genres        []string `json:"genres"`
			Ratings       struct {
				Value float64 `json:"value"`
			} `json:"ratings"`
			Images []struct {
				CoverType string `json:"coverType"`
				RemoteURL string `json:"remoteUrl"`
			} `json:"images"`
		} `json:"series"`
	}
	if err := client.get(ctx, path, client.sonarrKey, &result); err != nil {
		return nil, fmt.Errorf("read Sonarr calendar: %w", err)
	}
	items := make([]episode, 0, len(result))
	for _, entry := range result {
		if !entry.AirDate.IsZero() {
			items = append(items, episode{
				Title: entry.Series.Title, TvdbID: entry.Series.TvdbID,
				PosterURL: poster(entry.Series.Images), BackdropURL: image(entry.Series.Images, "fanart"),
				SeasonNumber: entry.SeasonNumber, EpisodeNumber: entry.EpisodeNumber,
				EpisodeTitle: entry.Title, AirDate: entry.AirDate,
				Overview: entry.Series.Overview, Year: entry.Series.Year, Runtime: entry.Series.Runtime,
				Studio: entry.Series.Network, Status: entry.Series.Status,
				Certification: entry.Series.Certification, Genres: entry.Series.Genres,
				Rating: entry.Series.Ratings.Value,
			})
		}
	}
	return items, nil
}

// image picks one cover type out of what Radarr/Sonarr scraped. Same proxying
// rationale as poster below.
func image(images []struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
}, coverType string) string {
	for _, candidate := range images {
		if candidate.CoverType == coverType {
			return artwork.ProxyURL(candidate.RemoteURL)
		}
	}
	return ""
}

func poster(images []struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
}) string {
	for _, image := range images {
		if image.CoverType == "poster" {
			// Radarr/Sonarr hand back the CDN URL they scraped. It goes
			// through our own origin: the proxy normalises the scheme, so the
			// plain http:// these sometimes return stops being mixed content,
			// and the browser never talks to the CDN directly.
			return artwork.ProxyURL(image.RemoteURL)
		}
	}
	return ""
}

func (client *Client) movieDates(ctx context.Context, tmdbID int, fallback movie) (releaseDates, error) {
	if client.tmdbKey == "" {
		return releaseDates{cinema: fallback.Cinema, digital: fallback.Digital}, nil
	}
	var response struct {
		Results []struct {
			Country string `json:"iso_3166_1"`
			Dates   []struct {
				Type int       `json:"type"`
				Date time.Time `json:"release_date"`
			} `json:"release_dates"`
		} `json:"results"`
	}
	path := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/release_dates", tmdbID)
	if err := client.get(ctx, path, client.tmdbKey, &response); err != nil {
		// A calendar remains useful during a temporary TMDB outage. Radarr's
		// dates are less reliably regional, but are safer than hiding every
		// movie from the dashboard.
		return releaseDates{cinema: fallback.Cinema, digital: fallback.Digital}, nil
	}
	var usFallback releaseDates
	for _, country := range response.Results {
		if country.Country != client.region && country.Country != "US" {
			continue
		}
		var cinema, digital time.Time
		for _, date := range country.Dates {
			switch date.Type {
			case 3:
				if cinema.IsZero() || date.Date.Before(cinema) {
					cinema = date.Date
				}
			case 2:
				if cinema.IsZero() || date.Date.Before(cinema) {
					cinema = date.Date
				}
			case 4:
				if digital.IsZero() || date.Date.Before(digital) {
					digital = date.Date
				}
			}
		}
		dates := releaseDates{cinema: cinema, digital: digital}
		if country.Country == client.region {
			return dates, nil
		}
		usFallback = dates
	}
	return usFallback, nil
}

func (client *Client) findTMDBTV(ctx context.Context, tvdbID int) (string, error) {
	if tvdbID == 0 || client.tmdbKey == "" {
		return "", nil
	}
	var response struct {
		Results []struct {
			ID int `json:"id"`
		} `json:"tv_results"`
	}
	path := fmt.Sprintf("https://api.themoviedb.org/3/find/%d?external_source=tvdb_id", tvdbID)
	if err := client.get(ctx, path, client.tmdbKey, &response); err != nil {
		return "", nil
	}
	if len(response.Results) == 0 {
		return "", nil
	}
	return fmt.Sprintf("%d", response.Results[0].ID), nil
}

// isTMDBv4Token distinguishes a v4 "API Read Access Token" (a JWT: three
// base64 segments joined by dots) from a legacy v3 API key (a 32-char hex
// string). Only v4 tokens work with Bearer auth; v3 keys must go in the
// query string, which is TMDB's only auth method for that key type.
func isTMDBv4Token(key string) bool {
	return strings.Count(key, ".") == 2 && len(key) > 40
}

func (client *Client) get(ctx context.Context, endpoint, apiKey string, target any) error {
	isTMDB := strings.Contains(endpoint, "themoviedb.org")
	v4 := isTMDB && isTMDBv4Token(apiKey)
	if isTMDB && !v4 {
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		endpoint += separator + "api_key=" + url.QueryEscape(apiKey)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if isTMDB {
		if v4 {
			request.Header.Set("Authorization", "Bearer "+apiKey)
		}
	} else {
		request.Header.Set("X-Api-Key", apiKey)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("returned %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(target)
}

// Detail returns the calendar entry a source knows by id. It exists so a
// detail screen works without Seerr and without TMDB: everything it returns
// was already in the calendar response the list was built from, so this walks
// the same two lists rather than calling Radarr or Sonarr again.
//
// Matching is by (source, id) rather than by TMDB id alone, because a Sonarr
// entry has no TMDB id unless TMDB is configured.
func (client *Client) Detail(ctx context.Context, source, id string) (Item, bool, error) {
	if client == nil || id == "" || (source != SourceRadarr && source != SourceSonarr) {
		return Item{}, false, nil
	}
	upcoming, err := client.Upcoming(ctx)
	if err != nil {
		return Item{}, false, err
	}
	cinemas, err := client.InCinemas(ctx)
	if err != nil {
		return Item{}, false, err
	}
	for _, list := range [][]Item{upcoming, cinemas} {
		for _, item := range list {
			if item.Source == source && item.DetailID == id {
				return item, true, nil
			}
		}
	}
	// An entry may also be addressed by the TMDB id the tile carries, which is
	// what the frontend has when TMDB is configured. Still scoped to the
	// source: a Sonarr id must never resolve through Radarr's list.
	for _, list := range [][]Item{upcoming, cinemas} {
		for _, item := range list {
			if item.Source == source && item.TmdbID != "" && item.TmdbID == id {
				return item, true, nil
			}
		}
	}
	return Item{}, false, nil
}
