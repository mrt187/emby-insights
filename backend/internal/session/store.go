package session

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mrt187/EmbyInsights/internal/emby"
	"github.com/redis/go-redis/v9"
)

const Duration = 24 * time.Hour

type Store interface {
	Create(context.Context, emby.Identity) (string, error)
	Get(context.Context, string) (emby.Identity, error)
	Delete(context.Context, string) error
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(client *redis.Client) *RedisStore {
	return &RedisStore{client: client}
}

func (store *RedisStore) Create(ctx context.Context, identity emby.Identity) (string, error) {
	identifier := make([]byte, 32)
	if _, err := rand.Read(identifier); err != nil {
		return "", fmt.Errorf("generate session identifier: %w", err)
	}

	sessionID := base64.RawURLEncoding.EncodeToString(identifier)
	payload, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	if err := store.client.Set(ctx, key(sessionID), payload, Duration).Err(); err != nil {
		return "", err
	}
	return sessionID, nil
}

func (store *RedisStore) Get(ctx context.Context, sessionID string) (emby.Identity, error) {
	payload, err := store.client.Get(ctx, key(sessionID)).Bytes()
	if err != nil {
		return emby.Identity{}, err
	}

	var identity emby.Identity
	if err := json.Unmarshal(payload, &identity); err != nil {
		return emby.Identity{}, err
	}
	return identity, nil
}

func (store *RedisStore) Delete(ctx context.Context, sessionID string) error {
	return store.client.Del(ctx, key(sessionID)).Err()
}

func key(sessionID string) string {
	return "emby-insights:session:" + sessionID
}
