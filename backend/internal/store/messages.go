package store

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Message is one line in a user's chat thread with the admin. EmbyUserID
// (not exposed here) always identifies whose thread a row belongs to, even
// for admin replies — the admin has no thread of their own.
type Message struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	FromAdmin bool      `json:"fromAdmin"`
	CreatedAt time.Time `json:"createdAt"`
}

// MessageThread summarizes one user's conversation for the admin inbox.
type MessageThread struct {
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName"`
	LastMessage string    `json:"lastMessage"`
	LastAt      time.Time `json:"lastAt"`
	UnreadCount int       `json:"unreadCount"`
}

type MessageStore interface {
	Thread(ctx context.Context, embyUserID string) ([]Message, error)
	Send(ctx context.Context, embyUserID, displayName, body string, fromAdmin bool) error
	MarkRead(ctx context.Context, embyUserID string, fromAdminMessages bool) error
	UnreadCountForUser(ctx context.Context, embyUserID string) (int, error)
	UnreadCountForAdmin(ctx context.Context) (int, error)
	Threads(ctx context.Context) ([]MessageThread, error)
	DeleteThread(ctx context.Context, embyUserID string) error
}

type PostgresMessageStore struct {
	pool *pgxpool.Pool
}

func NewPostgresMessageStore(pool *pgxpool.Pool) *PostgresMessageStore {
	return &PostgresMessageStore{pool: pool}
}

// Thread returns every message in one user's conversation, oldest first.
func (store *PostgresMessageStore) Thread(ctx context.Context, embyUserID string) ([]Message, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, body, from_admin, created_at
		FROM messages
		WHERE emby_user_id = $1
		ORDER BY created_at ASC
	`, embyUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.Body, &message.FromAdmin, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

// Send appends one message to a user's thread. displayName seeds/refreshes
// the thread's cached name whenever the caller has one to offer (a user's
// own DisplayName, or the name the admin picked from the Emby directory to
// start a new thread); when empty, it falls back to whatever name is
// already stored for that thread, since there is no user directory lookup
// available at read time otherwise.
func (store *PostgresMessageStore) Send(ctx context.Context, embyUserID, displayName, body string, fromAdmin bool) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO messages (emby_user_id, body, from_admin, display_name)
		VALUES ($1, $2, $3, COALESCE(NULLIF($4, ''), (SELECT display_name FROM messages WHERE emby_user_id = $1 ORDER BY created_at DESC LIMIT 1), ''))
	`, embyUserID, body, fromAdmin, displayName)
	return err
}

// MarkRead clears read_at on the unread half of one user's thread.
// fromAdminMessages selects which half: true marks admin replies read (the
// user viewing their own thread), false marks the user's messages read (the
// admin viewing that user's thread).
func (store *PostgresMessageStore) MarkRead(ctx context.Context, embyUserID string, fromAdminMessages bool) error {
	_, err := store.pool.Exec(ctx, `
		UPDATE messages SET read_at = CURRENT_TIMESTAMP
		WHERE emby_user_id = $1 AND from_admin = $2 AND read_at IS NULL
	`, embyUserID, fromAdminMessages)
	return err
}

// UnreadCountForUser counts unread admin replies in one user's thread.
func (store *PostgresMessageStore) UnreadCountForUser(ctx context.Context, embyUserID string) (int, error) {
	var count int
	err := store.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE emby_user_id = $1 AND from_admin AND read_at IS NULL
	`, embyUserID).Scan(&count)
	return count, err
}

// UnreadCountForAdmin counts unread user messages across every thread.
func (store *PostgresMessageStore) UnreadCountForAdmin(ctx context.Context) (int, error) {
	var count int
	err := store.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE NOT from_admin AND read_at IS NULL
	`).Scan(&count)
	return count, err
}

// Threads lists one row per user with a message, most recently active
// first, for the admin inbox view.
func (store *PostgresMessageStore) Threads(ctx context.Context) ([]MessageThread, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT DISTINCT ON (emby_user_id)
			emby_user_id, display_name, body, created_at,
			(SELECT COUNT(*) FROM messages unread
				WHERE unread.emby_user_id = messages.emby_user_id AND NOT unread.from_admin AND unread.read_at IS NULL)
		FROM messages
		ORDER BY emby_user_id, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []MessageThread
	for rows.Next() {
		var thread MessageThread
		if err := rows.Scan(&thread.UserID, &thread.DisplayName, &thread.LastMessage, &thread.LastAt, &thread.UnreadCount); err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// DISTINCT ON requires ordering by emby_user_id first, so the most
	// recently active thread isn't necessarily first in the query result.
	sort.Slice(threads, func(i, j int) bool { return threads[i].LastAt.After(threads[j].LastAt) })
	return threads, nil
}

// DeleteThread removes every message in one user's thread — admin-only, and
// irreversible; there is no undo or archive.
func (store *PostgresMessageStore) DeleteThread(ctx context.Context, embyUserID string) error {
	_, err := store.pool.Exec(ctx, `DELETE FROM messages WHERE emby_user_id = $1`, embyUserID)
	return err
}
