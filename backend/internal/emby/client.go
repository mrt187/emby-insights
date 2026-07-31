package emby

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Credentials struct{ Username, Password string }

type Identity struct {
	UserID      string
	DisplayName string
	AccessToken string
	ServerID    string
}

type Authenticator interface {
	Authenticate(context.Context, Credentials) (Identity, error)
}

type AvatarReader interface {
	UserPrimaryImage(context.Context, Identity) (UserImage, error)
}

type FavoriteWriter interface {
	SetFavorite(ctx context.Context, userID, itemID string, favorite bool) error
}

// SetFavorite adds or removes an item from the user's Emby favorites — the
// first write operation against Emby other than Authenticate. Favorites are
// deliberately kept native rather than duplicated into our own database, so
// they stay in sync with every other Emby client.
//
// Both IDs are path-escaped: this request carries the admin API key, so an
// unescaped ID could be used to reach any other Emby endpoint with admin
// rights. Callers validate the ID format on top of this.
func (client *Client) SetFavorite(ctx context.Context, userID, itemID string, favorite bool) error {
	method := http.MethodPost
	if !favorite {
		method = http.MethodDelete
	}
	endpoint := client.baseURL + "/Users/" + url.PathEscape(userID) + "/FavoriteItems/" + url.PathEscape(itemID)
	request, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call Emby favorite: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("Emby favorite returned %s", response.Status)
	}
	return nil
}

type UserImage struct {
	ContentType string
	Data        []byte
}

type Client struct {
	baseURL     string
	deviceID    string
	adminAPIKey string
	httpClient  *http.Client
}

func NewClient(baseURL, deviceID, adminAPIKey string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		deviceID:    deviceID,
		adminAPIKey: adminAPIKey,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (client *Client) Authenticate(ctx context.Context, credentials Credentials) (Identity, error) {
	body, err := json.Marshal(struct {
		Username string `json:"Username"`
		Password string `json:"Pw"`
	}{credentials.Username, credentials.Password})
	if err != nil {
		return Identity{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/Users/AuthenticateByName", bytes.NewReader(body))
	if err != nil {
		return Identity{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Emby-Authorization", fmt.Sprintf("Emby Client=\"Emby Insights\", Device=\"Emby Insights\", DeviceId=\"%s\", Version=\"0.4.0\"", client.deviceID))
	response, err := client.httpClient.Do(request)
	if err != nil {
		return Identity{}, fmt.Errorf("call Emby login: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return Identity{}, ErrInvalidCredentials
	}
	var result struct {
		User struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"User"`
		AccessToken string `json:"AccessToken"`
		ServerID    string `json:"ServerId"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Identity{}, fmt.Errorf("decode Emby login response: %w", err)
	}
	if result.User.ID == "" || result.AccessToken == "" {
		return Identity{}, fmt.Errorf("Emby login response is incomplete")
	}
	return Identity{UserID: result.User.ID, DisplayName: result.User.Name, AccessToken: result.AccessToken, ServerID: result.ServerID}, nil
}

func (client *Client) UserPrimaryImage(ctx context.Context, identity Identity) (UserImage, error) {
	return client.userPrimaryImage(ctx, identity.UserID, identity.AccessToken)
}

func (client *Client) userPrimaryImage(ctx context.Context, userID, token string) (UserImage, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/Users/"+url.PathEscape(userID)+"/Images/Primary", nil)
	if err != nil {
		return UserImage{}, err
	}
	request.Header.Set("X-Emby-Token", token)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return UserImage{}, fmt.Errorf("fetch Emby profile image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return UserImage{}, ErrPrimaryImageUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return UserImage{}, fmt.Errorf("Emby profile image returned %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return UserImage{}, fmt.Errorf("read Emby profile image: %w", err)
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return UserImage{ContentType: contentType, Data: data}, nil
}

// ImageURL builds a link to our own image proxy (see server.itemImage)
// instead of exposing the Emby server's address to the browser. Item
// posters/backdrops used to be linked straight to client.baseURL, which
// only resolves for browsers on the same network as Emby — anyone reaching
// the dashboard through a reverse proxy from outside that network got a
// broken image instead.
func ImageURL(itemID, imageType, tag string, maxWidth int) string {
	return fmt.Sprintf("/api/images?itemId=%s&type=%s&tag=%s&maxWidth=%d", url.QueryEscape(itemID), imageType, url.QueryEscape(tag), maxWidth)
}

// ItemImage fetches an item's poster/backdrop with the admin API key —
// these aren't user-specific, so there's no per-user token to use.
func (client *Client) ItemImage(ctx context.Context, itemID, imageType, tag string, maxWidth int) (UserImage, error) {
	endpoint := fmt.Sprintf("%s/Items/%s/Images/%s?tag=%s&maxWidth=%d", client.baseURL, url.PathEscape(itemID), url.PathEscape(imageType), url.QueryEscape(tag), maxWidth)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return UserImage{}, err
	}
	request.Header.Set("X-Emby-Token", client.adminAPIKey)
	response, err := client.httpClient.Do(request)
	if err != nil {
		return UserImage{}, fmt.Errorf("fetch Emby item image: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return UserImage{}, ErrItemImageUnavailable
	}
	if response.StatusCode != http.StatusOK {
		return UserImage{}, fmt.Errorf("Emby item image returned %s", response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return UserImage{}, fmt.Errorf("read Emby item image: %w", err)
	}
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	return UserImage{ContentType: contentType, Data: data}, nil
}
