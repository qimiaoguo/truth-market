package repository

import (
	"context"
	"time"
)

// SessionStore defines operations for managing user sessions, typically backed
// by Redis or another in-memory store with TTL support.
type SessionStore interface {
	// Set stores a session token mapped to a user ID with the given
	// time-to-live. If a session with the same token already exists, it is
	// overwritten.
	Set(ctx context.Context, token string, userID string, ttl time.Duration) error

	// Get retrieves the user ID associated with the given session token. It
	// returns an error if the token does not exist or has expired.
	Get(ctx context.Context, token string) (string, error)

	// Delete removes the session identified by the given token.
	Delete(ctx context.Context, token string) error
}
