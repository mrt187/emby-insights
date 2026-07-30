-- Repurposes the unused notifications table (created in 001, never
-- referenced by any Go code) as the store for the internal user<->admin
-- chat: same shape (per-user rows with read_at/created_at), just adding
-- the fields a two-way thread needs.
ALTER TABLE notifications RENAME TO messages;
ALTER INDEX notifications_user_unread_idx RENAME TO messages_user_unread_idx;
ALTER TABLE messages DROP COLUMN title;
ALTER TABLE messages ALTER COLUMN kind SET DEFAULT 'chat';
ALTER TABLE messages ADD COLUMN from_admin BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE messages ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ALTER COLUMN id SET DEFAULT gen_random_uuid();
