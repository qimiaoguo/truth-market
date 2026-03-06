package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

// SessionStore implements repository.SessionStore using Redis for session
// storage with automatic expiration via TTL.
type SessionStore struct {
	client *redis.Client
}

// NewSessionStore returns a new SessionStore backed by the given Redis client.
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// Set stores a session token mapped to a user ID with the given time-to-live.
func (s *SessionStore) Set(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, sessionKeyPrefix+token, userID, ttl).Err()
}

// Get retrieves the user ID associated with the given session token.
func (s *SessionStore) Get(ctx context.Context, token string) (string, error) {
	val, err := s.client.Get(ctx, sessionKeyPrefix+token).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("session not found")
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Delete removes the session identified by the given token.
func (s *SessionStore) Delete(ctx context.Context, token string) error {
	return s.client.Del(ctx, sessionKeyPrefix+token).Err()
}
