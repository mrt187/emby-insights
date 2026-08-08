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

// nextUpForSeries calls Emby's native /Shows/NextUp scoped to one series via
// SeriesId. The unscoped, all-series form of this endpoint only returns
// series Emby considers "actively watched" (driven by real playback/watch
// history) and comes back empty for series whose episodes were marked
// watched in bulk rather than actually played — which is common here, since
// users mark whole seasons watched from this app. Scoping by SeriesId
// sidesteps that and reliably returns the next unplayed episode regardless
// of how earlier ones were marked watched.
func (client *Client) nextUpForSeries(ctx context.Context, userID, seriesID string) (nextUpEpisode, bool, error) {
	query := url.Values{
		"UserId":   {userID},
		"SeriesId": {seriesID},
		"Limit":    {"1"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Shows/NextUp?"+query.Encode(), nil)
	if err != nil {
		return nextUpEpisode{}, false, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nextUpEpisode{}, false, fmt.Errorf("call Emby next up: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nextUpEpisode{}, false, fmt.Errorf("Emby next up returned %s", response.Status)
	}

	var result struct {
		Items []struct {
			Name              string `json:"Name"`
			IndexNumber       int    `json:"IndexNumber"`
			ParentIndexNumber int    `json:"ParentIndexNumber"`
		} `json:"Items"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nextUpEpisode{}, false, fmt.Errorf("decode Emby next up: %w", err)
	}
	if len(result.Items) == 0 {
		return nextUpEpisode{}, false, nil
	}
	item := result.Items[0]
	return nextUpEpisode{
		SeasonNumber:  item.ParentIndexNumber,
		EpisodeNumber: item.IndexNumber,
		EpisodeTitle:  item.Name,
	}, true, nil
}
