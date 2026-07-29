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
}

type MediaDetailReader interface {
	EmbyMediaDetail(ctx context.Context, userID, itemID string) (MediaDetail, error)
}

// EmbyMediaDetail reads everything a detail screen needs from a single Emby
// item lookup: description, genres, ratings, cast/crew and — for series —
// the personal watch progress via UserData.UnplayedItemCount.
func (client *Client) EmbyMediaDetail(ctx context.Context, userID, itemID string) (MediaDetail, error) {
	query := url.Values{
		"Fields": {"Overview,Genres,People,CommunityRating,OfficialRating,RecursiveItemCount"},
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
		ImageTags          struct {
			Primary string `json:"Primary"`
		} `json:"ImageTags"`
		BackdropImageTags []string `json:"BackdropImageTags"`
		People            []struct {
			Id              string `json:"Id"`
			Name            string `json:"Name"`
			Role            string `json:"Role"`
			Type            string `json:"Type"`
			PrimaryImageTag string `json:"PrimaryImageTag"`
		} `json:"People"`
		UserData struct {
			Played            bool `json:"Played"`
			UnplayedItemCount int  `json:"UnplayedItemCount"`
		} `json:"UserData"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return MediaDetail{}, fmt.Errorf("decode Emby item detail: %w", err)
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
		Played:          result.UserData.Played,
	}
	if result.ImageTags.Primary != "" {
		detail.PosterURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=600", client.baseURL, result.Id, result.ImageTags.Primary)
	}
	if len(result.BackdropImageTags) > 0 {
		detail.BackdropURL = fmt.Sprintf("%s/Items/%s/Images/Backdrop?tag=%s&maxWidth=1600", client.baseURL, result.Id, result.BackdropImageTags[0])
	}
	if detail.IsSeries {
		detail.TotalEpisodes = result.RecursiveItemCount
		detail.WatchedEpisodes = result.RecursiveItemCount - result.UserData.UnplayedItemCount
	}

	for _, person := range result.People {
		var imageURL string
		if person.PrimaryImageTag != "" {
			imageURL = fmt.Sprintf("%s/Items/%s/Images/Primary?tag=%s&maxWidth=200", client.baseURL, person.Id, person.PrimaryImageTag)
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
