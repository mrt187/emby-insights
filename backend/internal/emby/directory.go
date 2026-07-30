package emby

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// EmbyUser is one entry in the server's user directory. It exists only so
// the admin can pick who to start a new chat thread with — it is never
// shown to regular users.
type EmbyUser struct {
	ID   string
	Name string
}

type UserDirectoryReader interface {
	Users(ctx context.Context) ([]EmbyUser, error)
}

func (client *Client) Users(ctx context.Context) ([]EmbyUser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call Emby users: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Emby users returned %s", response.Status)
	}

	var raw []struct {
		ID   string `json:"Id"`
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode Emby users: %w", err)
	}

	users := make([]EmbyUser, 0, len(raw))
	for _, entry := range raw {
		users = append(users, EmbyUser{ID: entry.ID, Name: entry.Name})
	}
	return users, nil
}

type AdminAvatarReader interface {
	UserPrimaryImageByID(ctx context.Context, userID string) (UserImage, error)
}

// UserPrimaryImageByID fetches any user's avatar with the admin API key,
// rather than that user's own access token — needed for the user picker,
// which must show people who have never logged into this app.
func (client *Client) UserPrimaryImageByID(ctx context.Context, userID string) (UserImage, error) {
	return client.userPrimaryImage(ctx, userID, client.adminAPIKey)
}
