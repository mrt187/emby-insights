package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PushSubscription is one browser's Web Push registration.
type PushSubscription struct {
	Endpoint string
	P256dh   string
	Auth     string
}

type PushSubscriptionStore interface {
	// Save upserts by endpoint: resubscribing the same browser refreshes its
	// keys/last-used timestamp instead of creating a duplicate row.
	Save(ctx context.Context, embyUserID, endpoint, p256dh, auth string) error
	// Delete removes one user's subscription by endpoint. Scoped to
	// embyUserID so one user can't unsubscribe another user's device by
	// guessing or observing their endpoint.
	Delete(ctx context.Context, embyUserID, endpoint string) error
	// ForUser returns every subscription (device/browser) registered for one
	// user, for fan-out delivery.
	ForUser(ctx context.Context, embyUserID string) ([]PushSubscription, error)
}

type PostgresPushSubscriptionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresPushSubscriptionStore(pool *pgxpool.Pool) *PostgresPushSubscriptionStore {
	return &PostgresPushSubscriptionStore{pool: pool}
}

func (store *PostgresPushSubscriptionStore) Save(ctx context.Context, embyUserID, endpoint, p256dh, auth string) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO push_subscriptions (emby_user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE SET
			emby_user_id = EXCLUDED.emby_user_id,
			p256dh = EXCLUDED.p256dh,
			auth = EXCLUDED.auth,
			last_used_at = CURRENT_TIMESTAMP
	`, embyUserID, endpoint, p256dh, auth)
	return err
}

func (store *PostgresPushSubscriptionStore) Delete(ctx context.Context, embyUserID, endpoint string) error {
	_, err := store.pool.Exec(ctx, `DELETE FROM push_subscriptions WHERE endpoint = $1 AND emby_user_id = $2`, endpoint, embyUserID)
	return err
}

func (store *PostgresPushSubscriptionStore) ForUser(ctx context.Context, embyUserID string) ([]PushSubscription, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT endpoint, p256dh, auth FROM push_subscriptions WHERE emby_user_id = $1
	`, embyUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subscriptions []PushSubscription
	for rows.Next() {
		var subscription PushSubscription
		if err := rows.Scan(&subscription.Endpoint, &subscription.P256dh, &subscription.Auth); err != nil {
			return nil, err
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, rows.Err()
}

// PushSeenStore tracks which "New For You" items a user has already been
// pushed a notification for, so the background poller doesn't re-notify on
// every tick for the same item.
type PushSeenStore interface {
	// UnseenItemIDs filters itemIDs down to the ones not yet recorded as seen
	// for embyUserID.
	UnseenItemIDs(ctx context.Context, embyUserID string, itemIDs []string) ([]string, error)
	// MarkSeen records itemIDs as seen for embyUserID (idempotent).
	MarkSeen(ctx context.Context, embyUserID string, itemIDs []string) error
	// DistinctUserIDs lists every user with at least one push subscription —
	// the poller only needs to check users who could actually receive a push.
	DistinctUserIDs(ctx context.Context) ([]string, error)
}

type PostgresPushSeenStore struct {
	pool *pgxpool.Pool
}

func NewPostgresPushSeenStore(pool *pgxpool.Pool) *PostgresPushSeenStore {
	return &PostgresPushSeenStore{pool: pool}
}

func (store *PostgresPushSeenStore) UnseenItemIDs(ctx context.Context, embyUserID string, itemIDs []string) ([]string, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	rows, err := store.pool.Query(ctx, `
		SELECT item_id FROM unnest($2::text[]) AS item_id
		WHERE NOT EXISTS (
			SELECT 1 FROM push_seen_items
			WHERE emby_user_id = $1 AND push_seen_items.item_id = item_id
		)
	`, embyUserID, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var unseen []string
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		unseen = append(unseen, itemID)
	}
	return unseen, rows.Err()
}

func (store *PostgresPushSeenStore) MarkSeen(ctx context.Context, embyUserID string, itemIDs []string) error {
	if len(itemIDs) == 0 {
		return nil
	}
	_, err := store.pool.Exec(ctx, `
		INSERT INTO push_seen_items (emby_user_id, item_id)
		SELECT $1, item_id FROM unnest($2::text[]) AS item_id
		ON CONFLICT (emby_user_id, item_id) DO NOTHING
	`, embyUserID, itemIDs)
	return err
}

func (store *PostgresPushSeenStore) DistinctUserIDs(ctx context.Context) ([]string, error) {
	rows, err := store.pool.Query(ctx, `SELECT DISTINCT emby_user_id FROM push_subscriptions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, rows.Err()
}
