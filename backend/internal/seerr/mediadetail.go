package seerr

import (
	"context"
	"fmt"
)

type Person struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	ImageURL string `json:"imageUrl"`
}

type MediaDetail struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Overview        string   `json:"overview"`
	PosterURL       string   `json:"posterUrl"`
	BackdropURL     string   `json:"backdropUrl"`
	Genres          []string `json:"genres"`
	CommunityRating float64  `json:"communityRating"`
	Year            int      `json:"year"`
	RuntimeMinutes  int      `json:"runtimeMinutes"`
	Cast            []Person `json:"cast"`
	Crew            []Person `json:"crew"`
}

type MediaDetailReader interface {
	MediaDetail(ctx context.Context, mediaType string, tmdbID int) (MediaDetail, error)
}

const castCrewLimit = 10

// MediaDetail reads a movie or TV show's TMDB metadata through Seerr's
// proxy — used for posters that are not (yet) in the Emby library, such as
// open requests and discover lists.
func (client *Client) MediaDetail(ctx context.Context, mediaType string, tmdbID int) (MediaDetail, error) {
	if client == nil {
		return MediaDetail{}, fmt.Errorf("Seerr is not configured")
	}

	var result struct {
		Title        string  `json:"title"` // present on movies
		Name         string  `json:"name"`  // present on tv shows
		Overview     string  `json:"overview"`
		PosterPath   string  `json:"posterPath"`
		BackdropPath string  `json:"backdropPath"`
		VoteAverage  float64 `json:"voteAverage"`
		ReleaseDate  string  `json:"releaseDate"`  // movies
		FirstAirDate string  `json:"firstAirDate"` // tv shows
		Runtime      int     `json:"runtime"`      // movies
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
		Credits struct {
			Cast []struct {
				Name        string `json:"name"`
				Character   string `json:"character"`
				ProfilePath string `json:"profilePath"`
			} `json:"cast"`
			Crew []struct {
				Name        string `json:"name"`
				Job         string `json:"job"`
				ProfilePath string `json:"profilePath"`
			} `json:"crew"`
		} `json:"credits"`
	}

	path := fmt.Sprintf("/api/v1/%s/%d", mediaType, tmdbID)
	if _, err := client.get(ctx, path, &result); err != nil {
		return MediaDetail{}, err
	}

	title := result.Title
	if title == "" {
		title = result.Name
	}
	date := result.ReleaseDate
	if date == "" {
		date = result.FirstAirDate
	}
	year := 0
	if len(date) >= 4 {
		fmt.Sscanf(date[:4], "%d", &year)
	}

	detail := MediaDetail{
		ID:              fmt.Sprintf("%d", tmdbID),
		Title:           title,
		Overview:        result.Overview,
		CommunityRating: result.VoteAverage,
		Year:            year,
		RuntimeMinutes:  result.Runtime,
	}
	if result.PosterPath != "" {
		detail.PosterURL = posterBaseURL + result.PosterPath
	}
	if result.BackdropPath != "" {
		detail.BackdropURL = "https://image.tmdb.org/t/p/w1280" + result.BackdropPath
	}
	for _, genre := range result.Genres {
		detail.Genres = append(detail.Genres, genre.Name)
	}
	for index, cast := range result.Credits.Cast {
		if index >= castCrewLimit {
			break
		}
		var imageURL string
		if cast.ProfilePath != "" {
			imageURL = posterBaseURL + cast.ProfilePath
		}
		detail.Cast = append(detail.Cast, Person{Name: cast.Name, Role: cast.Character, ImageURL: imageURL})
	}
	for index, crew := range result.Credits.Crew {
		if index >= castCrewLimit {
			break
		}
		var imageURL string
		if crew.ProfilePath != "" {
			imageURL = posterBaseURL + crew.ProfilePath
		}
		detail.Crew = append(detail.Crew, Person{Name: crew.Name, Role: crew.Job, ImageURL: imageURL})
	}

	return detail, nil
}
