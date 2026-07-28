package emby

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	request.Header.Set("X-Emby-Authorization", fmt.Sprintf("Emby Client=\"Emby Insights\", Device=\"Emby Insights\", DeviceId=\"%s\", Version=\"0.1.0\"", client.deviceID))
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
