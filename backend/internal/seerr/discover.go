package seerr

import (
	"context"
	"strconv"
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

// discover reads a Seerr/TMDB discover list. Movies and TV entries use
// different field names for the same concepts (title/name,
// releaseDate/firstAirDate), so both are decoded and whichever is present
// wins.
func (client *Client) discover(ctx context.Context, path string) ([]DiscoverItem, error) {
	if client == nil {
		return nil, nil
	}

	var result struct {
		Results []struct {
			ID         int    `json:"id"`
			Title      string `json:"title"`
			Name       string `json:"name"`
			PosterPath string `json:"posterPath"`
			MediaType  string `json:"mediaType"`
		} `json:"results"`
	}
	if _, err := client.get(ctx, path+"?page=1", &result); err != nil {
		return nil, err
	}

	items := make([]DiscoverItem, 0, len(result.Results))
	for _, entry := range result.Results {
		title := entry.Title
		if title == "" {
			title = entry.Name
		}
		var posterURL string
		if entry.PosterPath != "" {
			posterURL = posterBaseURL + entry.PosterPath
		}
		items = append(items, DiscoverItem{
			ID:        strconv.Itoa(entry.ID),
			Title:     title,
			PosterURL: posterURL,
			MediaType: entry.MediaType,
		})
	}
	return items, nil
}
