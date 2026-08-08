-- Web Push subscriptions (browser PushSubscription objects). One user can
-- have several rows (one per device/browser); endpoint is unique so
-- resubscribing the same browser upserts instead of duplicating.
CREATE TABLE push_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    emby_user_id TEXT NOT NULL REFERENCES user_mappings (emby_user_id) ON DELETE CASCADE,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX push_subscriptions_user_idx ON push_subscriptions (emby_user_id);
