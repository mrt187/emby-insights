// Package comingsoon reads the release calendars directly from Radarr, Sonarr
// and TMDB. It deliberately does not create Emby library items.
package comingsoon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const posterBaseURL = "https://image.tmdb.org/t/p/w500"

type Item struct {
	ID               string `json:"id"`
	TmdbID           string `json:"tmdbId"`
	Title            string `json:"title"`
	PosterURL        string `json:"posterUrl"`
	MediaType        string `json:"mediaType"`
	AvailabilityDate string `json:"availabilityDate"`
	CinemaStartDate  string `json:"cinemaStartDate,omitempty"`
	CinemaEndDate    string `json:"cinemaEndDate,omitempty"`
	SeasonNumber     int    `json:"seasonNumber,omitempty"`
	EpisodeNumber    int    `json:"episodeNumber,omitempty"`
	EpisodeTitle     string `json:"episodeTitle,omitempty"`
}

type Reader interface {
	Upcoming(context.Context) ([]Item, error)
	InCinemas(context.Context) ([]Item, error)
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
			items = append(items, Item{ID: tmdbID, TmdbID: tmdbID, Title: episode.Title, PosterURL: episode.PosterURL, MediaType: "tv", AvailabilityDate: episode.AirDate.Format(time.RFC3339), SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, EpisodeTitle: episode.EpisodeTitle})
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
		if !dates.cinema.IsZero() && !dates.cinema.After(now.AddDate(0, 0, client.daysAhead)) && dates.digital.After(now) {
			items = append(items, movie.item(dates.cinema, dates.digital, "movie"))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CinemaStartDate < items[j].CinemaStartDate })
	return items, nil
}

type movie struct {
	Title, PosterURL string
	TmdbID           int
	Cinema, Digital  time.Time
}

func (movie movie) item(available, cinemaEnd time.Time, mediaType string) Item {
	id := fmt.Sprintf("%d", movie.TmdbID)
	return Item{ID: id, TmdbID: id, Title: movie.Title, PosterURL: movie.PosterURL, MediaType: mediaType, AvailabilityDate: available.Format(time.RFC3339), CinemaStartDate: available.Format(time.RFC3339), CinemaEndDate: cinemaEnd.Format(time.RFC3339)}
}

type episode struct {
	Title, PosterURL                    string
	TvdbID, SeasonNumber, EpisodeNumber int
	EpisodeTitle                        string
	AirDate                             time.Time
}
type releaseDates struct{ cinema, digital time.Time }

func (client *Client) movies(ctx context.Context) ([]movie, error) {
	if client.radarrURL == "" || client.radarrKey == "" {
		return nil, nil
	}
	today := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	end := time.Now().UTC().AddDate(0, 0, client.daysAhead+120).Format("2006-01-02")
	path := client.radarrURL + "/api/v3/calendar?start=" + today + "&end=" + end + "&unmonitored=false"
	var result []struct {
		Title          string    `json:"title"`
		TmdbID         int       `json:"tmdbId"`
		InCinemas      time.Time `json:"inCinemas"`
		DigitalRelease time.Time `json:"digitalRelease"`
		Images         []struct {
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
			items = append(items, movie{Title: entry.Title, TmdbID: entry.TmdbID, Cinema: entry.InCinemas, Digital: entry.DigitalRelease, PosterURL: poster(entry.Images)})
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
			Title  string `json:"title"`
			TvdbID int    `json:"tvdbId"`
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
			items = append(items, episode{Title: entry.Series.Title, TvdbID: entry.Series.TvdbID, PosterURL: poster(entry.Series.Images), SeasonNumber: entry.SeasonNumber, EpisodeNumber: entry.EpisodeNumber, EpisodeTitle: entry.Title, AirDate: entry.AirDate})
		}
	}
	return items, nil
}

func poster(images []struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
}) string {
	for _, image := range images {
		if image.CoverType == "poster" {
			return image.RemoteURL
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
	path := fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/release_dates?api_key=%s", tmdbID, url.QueryEscape(client.tmdbKey))
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
	path := fmt.Sprintf("https://api.themoviedb.org/3/find/%d?external_source=tvdb_id&api_key=%s", tvdbID, url.QueryEscape(client.tmdbKey))
	if err := client.get(ctx, path, client.tmdbKey, &response); err != nil {
		return "", nil
	}
	if len(response.Results) == 0 {
		return "", nil
	}
	return fmt.Sprintf("%d", response.Results[0].ID), nil
}

func (client *Client) get(ctx context.Context, endpoint, apiKey string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if !strings.Contains(endpoint, "themoviedb.org") {
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
