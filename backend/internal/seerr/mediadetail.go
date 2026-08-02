package seerr

import (
	"context"
	"fmt"
	"github.com/mrt187/EmbyInsights/internal/artwork"
)

type Person struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	ImageURL string `json:"imageUrl"`
}

type RequestableSeason struct {
	SeasonNumber int `json:"seasonNumber"`
	EpisodeCount int `json:"episodeCount"`
	// Available marks a season Seerr already reports as fully available
	// (status 5) for a partially-available show, so the frontend can hide
	// it from the season picker — only the seasons still missing should be
	// selectable to request.
	Available bool `json:"available,omitempty"`
	// Requested marks a season Seerr already has an open request for
	// (status 2 pending or 3 processing), so the frontend can hide it from
	// the season picker too — otherwise it would be re-requestable even
	// though it isn't "Available" yet.
	Requested bool `json:"requested,omitempty"`
}

type MediaDetail struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Overview        string              `json:"overview"`
	PosterURL       string              `json:"posterUrl"`
	BackdropURL     string              `json:"backdropUrl"`
	Genres          []string            `json:"genres"`
	CommunityRating float64             `json:"communityRating"`
	Year            int                 `json:"year"`
	RuntimeMinutes  int                 `json:"runtimeMinutes"`
	Cast            []Person            `json:"cast"`
	Crew            []Person            `json:"crew"`
	Seasons         []RequestableSeason `json:"seasons"`
	// Status is TMDB's own production status ("Released", "Returning
	// Series", ...), translated to German — distinct from MediaStatus below.
	Status string `json:"status"`
	// ReleaseDate is the raw ISO release/first-air date, so the frontend can
	// format it however it likes (Year above is only the parsed year).
	ReleaseDate string   `json:"releaseDate"`
	Studios     []string `json:"studios"`
	// MediaStatus is 0 when nobody has requested or added this title yet
	// (Seerr's own TMDB proxy omits "mediaInfo" entirely in that case) — the
	// frontend shows a request button only then. Non-zero values mirror
	// Seerr's own MediaStatus enum (5 = available).
	MediaStatus int `json:"mediaStatus"`
	// ImdbID is Seerr's own externalIds passthrough — never sent to the
	// frontend (json:"-"), only used server-side by server.go to enrich the
	// response with OMDb ratings before it goes out, since OMDb is keyed by
	// IMDb id rather than TMDB id.
	ImdbID string `json:"-"`
	// ImdbRating / RottenTomatoesRating come from OMDb (optional
	// integration), not TMDB — TMDB only exposes its own community score,
	// already carried above as CommunityRating. Both stay empty when OMDb
	// isn't configured or has no data for this title.
	ImdbRating           string `json:"imdbRating,omitempty"`
	RottenTomatoesRating string `json:"rottenTomatoesRating,omitempty"`
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
		Status       string  `json:"status"`
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
		ProductionCompanies []struct {
			Name string `json:"name"`
		} `json:"productionCompanies"`
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
		Seasons []struct {
			SeasonNumber int `json:"seasonNumber"`
			EpisodeCount int `json:"episodeCount"`
		} `json:"seasons"`
		MediaInfo *struct {
			Status  int `json:"status"`
			Seasons []struct {
				SeasonNumber int `json:"seasonNumber"`
				Status       int `json:"status"`
			} `json:"seasons"`
		} `json:"mediaInfo"`
		ExternalIDs *struct {
			ImdbID string `json:"imdbId"`
		} `json:"externalIds"`
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
		Status:          tmdbStatusLabel(result.Status),
		ReleaseDate:     date,
		Genres:          []string{},
		Studios:         []string{},
		Cast:            []Person{},
		Crew:            []Person{},
		Seasons:         []RequestableSeason{},
	}
	if result.ExternalIDs != nil {
		detail.ImdbID = result.ExternalIDs.ImdbID
	}

	seasonAvailability := map[int]int{}
	if result.MediaInfo != nil {
		detail.MediaStatus = result.MediaInfo.Status
		for _, season := range result.MediaInfo.Seasons {
			seasonAvailability[season.SeasonNumber] = season.Status
		}
	}
	for _, company := range result.ProductionCompanies {
		detail.Studios = append(detail.Studios, company.Name)
	}
	for _, season := range result.Seasons {
		if season.SeasonNumber == 0 { // "Specials" — not a real season to request
			continue
		}
		const (
			seerrStatusPending            = 2
			seerrStatusProcessing         = 3
			seerrStatusPartiallyAvailable = 4
			seerrStatusAvailable          = 5
		)
		status := seasonAvailability[season.SeasonNumber]
		detail.Seasons = append(detail.Seasons, RequestableSeason{
			SeasonNumber: season.SeasonNumber,
			EpisodeCount: season.EpisodeCount,
			Available:    status == seerrStatusAvailable,
			Requested:    status == seerrStatusPending || status == seerrStatusProcessing || status == seerrStatusPartiallyAvailable,
		})
	}
	if result.PosterPath != "" {
		detail.PosterURL = artwork.ProxyURL(posterBaseURL + result.PosterPath)
	}
	if result.BackdropPath != "" {
		detail.BackdropURL = artwork.ProxyURL("https://image.tmdb.org/t/p/w1280" + result.BackdropPath)
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
			imageURL = artwork.ProxyURL(posterBaseURL + cast.ProfilePath)
		}
		detail.Cast = append(detail.Cast, Person{Name: cast.Name, Role: cast.Character, ImageURL: imageURL})
	}
	for index, crew := range result.Credits.Crew {
		if index >= castCrewLimit {
			break
		}
		var imageURL string
		if crew.ProfilePath != "" {
			imageURL = artwork.ProxyURL(posterBaseURL + crew.ProfilePath)
		}
		detail.Crew = append(detail.Crew, Person{Name: crew.Name, Role: crew.Job, ImageURL: imageURL})
	}

	return detail, nil
}

// tmdbStatusLabel translates TMDB's own English production-status strings.
// Unrecognized values (TMDB adds new ones occasionally) pass through as-is.
func tmdbStatusLabel(status string) string {
	switch status {
	case "Released":
		return "Veröffentlicht"
	case "Post Production":
		return "In der Postproduktion"
	case "In Production":
		return "In Produktion"
	case "Planned":
		return "Geplant"
	case "Rumored":
		return "Gerücht"
	case "Canceled":
		return "Abgesetzt"
	case "Ended":
		return "Beendet"
	case "Returning Series":
		return "Laufende Serie"
	case "Pilot":
		return "Pilotfolge"
	default:
		return status
	}
}
