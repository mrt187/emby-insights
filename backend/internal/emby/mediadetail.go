package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Person struct {
	Name     string `json:"name"`
	Role     string `json:"role"`
	ImageURL string `json:"imageUrl"`
}

type Season struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PosterURL       string `json:"posterUrl"`
	IndexNumber     int    `json:"indexNumber"`
	WatchedEpisodes int    `json:"watchedEpisodes"`
	TotalEpisodes   int    `json:"totalEpisodes"`
	Played          bool   `json:"played"`
}

type MediaDetail struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Overview        string   `json:"overview"`
	PosterURL       string   `json:"posterUrl"`
	BackdropURL     string   `json:"backdropUrl"`
	Genres          []string `json:"genres"`
	CommunityRating float64  `json:"communityRating"`
	OfficialRating  string   `json:"officialRating"`
	Year            int      `json:"year"`
	RuntimeMinutes  int      `json:"runtimeMinutes"`
	Cast            []Person `json:"cast"`
	Crew            []Person `json:"crew"`
	IsSeries        bool     `json:"isSeries"`
	WatchedEpisodes int      `json:"watchedEpisodes"`
	TotalEpisodes   int      `json:"totalEpisodes"`
	Played          bool     `json:"played"`
	IsFavorite      bool     `json:"isFavorite"`
	Seasons         []Season `json:"seasons"`
	// CurrentSeasonNumber/CurrentEpisodeNumber are set when this detail was
	// opened for an in-progress episode (e.g. from "Weiterschauen") — Emby's
	// Continue Watching list only returns the episode's own ID, so the
	// detail is resolved up to its series, with these two carrying which
	// episode is actually in progress.
	CurrentSeasonNumber  int `json:"currentSeasonNumber,omitempty"`
	CurrentEpisodeNumber int `json:"currentEpisodeNumber,omitempty"`
	// ImdbID/TmdbID/TvdbID come from Emby's ProviderIds and stay server-side
	// (like seerr.MediaDetail.ImdbID): the browser has no use for them, but
	// they are what lets an Emby item be looked up in third-party services
	// keyed by external id.
	ImdbID string `json:"-"`
	TmdbID string `json:"-"`
	TvdbID string `json:"-"`
}

type MediaDetailReader interface {
	EmbyMediaDetail(ctx context.Context, userID, itemID string) (MediaDetail, error)
}

// EmbyMediaDetail reads everything a detail screen needs from a single Emby
// item lookup: description, genres, ratings, cast/crew and — for series —
// the personal watch progress via UserData.UnplayedItemCount.
func (client *Client) EmbyMediaDetail(ctx context.Context, userID, itemID string) (MediaDetail, error) {
	query := url.Values{
		"Fields": {"Overview,Genres,People,CommunityRating,OfficialRating,RecursiveItemCount,ProviderIds"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID+"/Items/"+itemID+"?"+query.Encode(), nil)
	if err != nil {
		return MediaDetail{}, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return MediaDetail{}, fmt.Errorf("call Emby item detail: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return MediaDetail{}, fmt.Errorf("Emby item detail returned %s", response.Status)
	}

	var result struct {
		Id                 string   `json:"Id"`
		Name               string   `json:"Name"`
		Type               string   `json:"Type"`
		Overview           string   `json:"Overview"`
		Genres             []string `json:"Genres"`
		CommunityRating    float64  `json:"CommunityRating"`
		OfficialRating     string   `json:"OfficialRating"`
		ProductionYear     int      `json:"ProductionYear"`
		RunTimeTicks       int64    `json:"RunTimeTicks"`
		RecursiveItemCount int      `json:"RecursiveItemCount"`
		SeriesId           string   `json:"SeriesId"`
		ParentIndexNumber  int      `json:"ParentIndexNumber"`
		IndexNumber        int      `json:"IndexNumber"`
		ImageTags          struct {
			Primary string `json:"Primary"`
		} `json:"ImageTags"`
		BackdropImageTags []string `json:"BackdropImageTags"`
		ProviderIds       struct {
			Imdb string `json:"Imdb"`
			Tmdb string `json:"Tmdb"`
			Tvdb string `json:"Tvdb"`
		} `json:"ProviderIds"`
		People []struct {
			Id              string `json:"Id"`
			Name            string `json:"Name"`
			Role            string `json:"Role"`
			Type            string `json:"Type"`
			PrimaryImageTag string `json:"PrimaryImageTag"`
		} `json:"People"`
		UserData struct {
			Played            bool `json:"Played"`
			UnplayedItemCount int  `json:"UnplayedItemCount"`
			IsFavorite        bool `json:"IsFavorite"`
		} `json:"UserData"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return MediaDetail{}, fmt.Errorf("decode Emby item detail: %w", err)
	}

	// Continue Watching only returns an episode's own ID; show its series
	// instead, carrying the in-progress season/episode number along.
	if result.Type == "Episode" && result.SeriesId != "" {
		detail, err := client.EmbyMediaDetail(ctx, userID, result.SeriesId)
		if err != nil {
			return MediaDetail{}, err
		}
		detail.CurrentSeasonNumber = result.ParentIndexNumber
		detail.CurrentEpisodeNumber = result.IndexNumber
		return detail, nil
	}

	if result.Genres == nil {
		result.Genres = []string{}
	}

	detail := MediaDetail{
		ID:              result.Id,
		Title:           result.Name,
		Overview:        result.Overview,
		Genres:          result.Genres,
		CommunityRating: result.CommunityRating,
		OfficialRating:  result.OfficialRating,
		Year:            result.ProductionYear,
		RuntimeMinutes:  int(result.RunTimeTicks / 10_000_000 / 60),
		IsSeries:        result.Type == "Series",
		ImdbID:          result.ProviderIds.Imdb,
		TmdbID:          result.ProviderIds.Tmdb,
		TvdbID:          result.ProviderIds.Tvdb,
		Played:          result.UserData.Played,
		IsFavorite:      result.UserData.IsFavorite,
		Cast:            []Person{},
		Crew:            []Person{},
		Seasons:         []Season{},
	}
	if result.ImageTags.Primary != "" {
		detail.PosterURL = ImageURL(result.Id, "Primary", result.ImageTags.Primary, 600)
	}
	if len(result.BackdropImageTags) > 0 {
		detail.BackdropURL = ImageURL(result.Id, "Backdrop", result.BackdropImageTags[0], 1600)
	}
	if detail.IsSeries {
		detail.TotalEpisodes = result.RecursiveItemCount
		detail.WatchedEpisodes = result.RecursiveItemCount - result.UserData.UnplayedItemCount
		seasons, err := client.seasons(ctx, userID, result.Id)
		if err != nil {
			return MediaDetail{}, err
		}
		detail.Seasons = seasons
	}

	for _, person := range result.People {
		var imageURL string
		if person.PrimaryImageTag != "" {
			imageURL = ImageURL(person.Id, "Primary", person.PrimaryImageTag, 200)
		}
		entry := Person{Name: person.Name, Role: person.Role, ImageURL: imageURL}
		if person.Type == "Actor" {
			detail.Cast = append(detail.Cast, entry)
		} else {
			entry.Role = person.Type
			detail.Crew = append(detail.Crew, entry)
		}
	}

	return detail, nil
}

func (client *Client) seasons(ctx context.Context, userID, seriesID string) ([]Season, error) {
	query := url.Values{
		"UserId": {userID},
		"Fields": {"RecursiveItemCount"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Shows/"+seriesID+"/Seasons?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby seasons: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby seasons returned %s", response.Status)
	}

	var result struct {
		Items []struct {
			Id                 string `json:"Id"`
			Name               string `json:"Name"`
			IndexNumber        int    `json:"IndexNumber"`
			RecursiveItemCount int    `json:"RecursiveItemCount"`
			ImageTags          struct {
				Primary string `json:"Primary"`
			} `json:"ImageTags"`
			UserData struct {
				Played            bool `json:"Played"`
				UnplayedItemCount int  `json:"UnplayedItemCount"`
			} `json:"UserData"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby seasons: %w", err)
	}

	seasons := make([]Season, 0, len(result.Items))
	for _, item := range result.Items {
		// Some seasons (e.g. "Specials") have no episodes in the library yet
		// and would show as "0 von 0" — skip those.
		if item.RecursiveItemCount == 0 {
			continue
		}
		var posterURL string
		if item.ImageTags.Primary != "" {
			posterURL = ImageURL(item.Id, "Primary", item.ImageTags.Primary, 300)
		}
		seasons = append(seasons, Season{
			ID:              item.Id,
			Title:           item.Name,
			PosterURL:       posterURL,
			IndexNumber:     item.IndexNumber,
			TotalEpisodes:   item.RecursiveItemCount,
			WatchedEpisodes: item.RecursiveItemCount - item.UserData.UnplayedItemCount,
			Played:          item.UserData.Played,
		})
	}
	return seasons, nil
}
