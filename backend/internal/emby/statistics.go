package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type PersonalWatchTime struct {
	WatchSeconds         int64  `json:"watchSeconds"`
	PreviousWatchSeconds int64  `json:"previousWatchSeconds"`
	PeriodStartsAt       string `json:"periodStartsAt"`
	PeriodEndsAt         string `json:"periodEndsAt"`
}

type PersonalStatisticsReader interface {
	PersonalWatchTime(context.Context, string, string) (PersonalWatchTime, error)
}

func (client *Client) PersonalWatchTime(ctx context.Context, userID, period string) (PersonalWatchTime, error) {
	query := url.Values{"UserId": {userID}, "Period": {period}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/EmbyInsights/PersonalStats?"+query.Encode(), nil)
	if err != nil {
		return PersonalWatchTime{}, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return PersonalWatchTime{}, fmt.Errorf("call Emby Insights personal statistics: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return PersonalWatchTime{}, fmt.Errorf("Emby Insights personal statistics returned %s", response.Status)
	}

	var statistics PersonalWatchTime
	if err := json.NewDecoder(response.Body).Decode(&statistics); err != nil {
		return PersonalWatchTime{}, fmt.Errorf("decode Emby Insights personal statistics: %w", err)
	}
	return statistics, nil
}
