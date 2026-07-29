package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type UserProfile struct {
	MemberSince    string `json:"memberSince"`
	LastActiveDate string `json:"lastActiveDate"`
	LastLoginDate  string `json:"lastLoginDate"`
}

type ProfileReader interface {
	UserProfile(ctx context.Context, userID string) (UserProfile, error)
}

// UserProfile reads account-level facts about the user themself (not their
// watch history) — when they joined and when they were last active.
func (client *Client) UserProfile(ctx context.Context, userID string) (UserProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+userID, nil)
	if err != nil {
		return UserProfile{}, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return UserProfile{}, fmt.Errorf("call Emby user profile: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return UserProfile{}, fmt.Errorf("Emby user profile returned %s", response.Status)
	}

	var result struct {
		DateCreated      string `json:"DateCreated"`
		LastActivityDate string `json:"LastActivityDate"`
		LastLoginDate    string `json:"LastLoginDate"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return UserProfile{}, fmt.Errorf("decode Emby user profile: %w", err)
	}
	return UserProfile{MemberSince: result.DateCreated, LastActiveDate: result.LastActivityDate, LastLoginDate: result.LastLoginDate}, nil
}
