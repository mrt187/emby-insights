package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// nextUpEpisode is the episode SeriesInProgress should point a user at next,
// per Emby's own "next up" concept — the first unplayed episode in a
// series' watch order.
type nextUpEpisode struct {
	SeasonNumber  int
	EpisodeNumber int
	EpisodeTitle  string
}

// nextUpBySeries calls Emby's native /Shows/NextUp, which already resolves
// "next unwatched episode" per series (season/episode gaps, specials
// ordering, etc.) — cheaper and more correct than re-deriving it from a
// full episode list per series. Limit is set well above Emby's default of
// 20, since this call spans every partially-watched series a user has, not
// one series' episodes.
func (client *Client) nextUpBySeries(ctx context.Context, userID string) (map[string]nextUpEpisode, error) {
	query := url.Values{
		"UserId": {userID},
		"Fields": {"PremiereDate"},
		"Limit":  {"500"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Shows/NextUp?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby next up: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby next up returned %s", response.Status)
	}

	var result struct {
		Items []struct {
			SeriesId          string `json:"SeriesId"`
			Name              string `json:"Name"`
			IndexNumber       int    `json:"IndexNumber"`
			ParentIndexNumber int    `json:"ParentIndexNumber"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Emby next up: %w", err)
	}

	byShow := make(map[string]nextUpEpisode, len(result.Items))
	for _, item := range result.Items {
		byShow[item.SeriesId] = nextUpEpisode{
			SeasonNumber:  item.ParentIndexNumber,
			EpisodeNumber: item.IndexNumber,
			EpisodeTitle:  item.Name,
		}
	}
	return byShow, nil
}
