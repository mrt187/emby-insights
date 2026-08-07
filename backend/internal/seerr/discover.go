package seerr

import (
	"context"
	"github.com/mrt187/EmbyInsights/internal/artwork"
	"net/url"
	"strconv"
	"strings"
)

type DiscoverItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	PosterURL string `json:"posterUrl"`
	MediaType string `json:"mediaType"`
}

type DiscoverReader interface {
	Trending(context.Context) ([]DiscoverItem, error)
	PopularMovies(context.Context) ([]DiscoverItem, error)
	PopularSeries(context.Context) ([]DiscoverItem, error)
	UpcomingMovies(context.Context) ([]DiscoverItem, error)
	UpcomingSeries(context.Context) ([]DiscoverItem, error)
	Search(ctx context.Context, query string) ([]DiscoverItem, error)
	DiscoverByProvider(ctx context.Context, providerID, region string) ([]DiscoverItem, error)
}

// DiscoverByProvider lists movies and series available on one streaming
// provider (a TMDB watch-provider ID, e.g. Netflix=8), for the "Streaming-
// Anbieter" row — Overseerr's discover endpoints mirror TMDB's own discover
// query parameters, including watchProviders/watchRegion.
func (client *Client) DiscoverByProvider(ctx context.Context, providerID, region string) ([]DiscoverItem, error) {
	if client == nil {
		return nil, nil
	}
	query := url.Values{"page": {"1"}, "watchProviders": {providerID}, "watchRegion": {region}}.Encode()
	movieEntries, err := client.discoverResults(ctx, "/api/v1/discover/movies?"+query)
	if err != nil {
		return nil, err
	}
	seriesEntries, err := client.discoverResults(ctx, "/api/v1/discover/tv?"+query)
	if err != nil {
		return nil, err
	}
	items := make([]DiscoverItem, 0, len(movieEntries)+len(seriesEntries))
	for _, entry := range movieEntries {
		items = append(items, entry.toDiscoverItem())
	}
	for _, entry := range seriesEntries {
		items = append(items, entry.toDiscoverItem())
	}
	return items, nil
}

func (client *Client) Trending(ctx context.Context) ([]DiscoverItem, error) {
	return client.discover(ctx, "/api/v1/discover/trending")
}

func (client *Client) PopularMovies(ctx context.Context) ([]DiscoverItem, error) {
	return client.discover(ctx, "/api/v1/discover/movies")
}

func (client *Client) PopularSeries(ctx context.Context) ([]DiscoverItem, error) {
	return client.discover(ctx, "/api/v1/discover/tv")
}

func (client *Client) UpcomingMovies(ctx context.Context) ([]DiscoverItem, error) {
	return client.discover(ctx, "/api/v1/discover/movies/upcoming")
}

func (client *Client) UpcomingSeries(ctx context.Context) ([]DiscoverItem, error) {
	return client.discover(ctx, "/api/v1/discover/tv/upcoming")
}

// Search looks up a free-text query directly against Seerr's own TMDB
// search, used for the "Suchen" button in the Anfragen tab — unlike the
// discover lists, its results can include mediaType "person" (e.g. actors),
// which are skipped since only movies/series can be requested.
func (client *Client) Search(ctx context.Context, query string) ([]DiscoverItem, error) {
	if client == nil {
		return nil, nil
	}
	// QueryEscape encodes spaces as "+", which Seerr's query parser (Express/qs)
	// does not decode back to a space — it reaches Seerr as a literal "+" and
	// the TMDB search behind it finds nothing. %20 is unambiguous everywhere.
	encodedQuery := strings.ReplaceAll(url.QueryEscape(query), "+", "%20")
	entries, err := client.discoverResults(ctx, "/api/v1/search?query="+encodedQuery)
	if err != nil {
		return nil, err
	}
	items := make([]DiscoverItem, 0, len(entries))
	for _, entry := range entries {
		if entry.MediaType != "movie" && entry.MediaType != "tv" {
			continue
		}
		items = append(items, entry.toDiscoverItem())
	}
	return items, nil
}

// discover reads a Seerr/TMDB discover list.
func (client *Client) discover(ctx context.Context, path string) ([]DiscoverItem, error) {
	if client == nil {
		return nil, nil
	}
	entries, err := client.discoverResults(ctx, path+"?page=1")
	if err != nil {
		return nil, err
	}
	items := make([]DiscoverItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, entry.toDiscoverItem())
	}
	return items, nil
}

type discoverEntry struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Name       string `json:"name"`
	PosterPath string `json:"posterPath"`
	MediaType  string `json:"mediaType"`
}

// toDiscoverItem converts a raw entry — movies and TV entries use different
// field names for the same concept (title vs. name), so both are decoded
// and whichever is present wins.
func (entry discoverEntry) toDiscoverItem() DiscoverItem {
	title := entry.Title
	if title == "" {
		title = entry.Name
	}
	var posterURL string
	if entry.PosterPath != "" {
		posterURL = artwork.ProxyURL(posterBaseURL + entry.PosterPath)
	}
	return DiscoverItem{ID: strconv.Itoa(entry.ID), Title: title, PosterURL: posterURL, MediaType: entry.MediaType}
}

func (client *Client) discoverResults(ctx context.Context, pathWithQuery string) ([]discoverEntry, error) {
	var result struct {
		Results []discoverEntry `json:"results"`
	}
	if _, err := client.get(ctx, pathWithQuery, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}
