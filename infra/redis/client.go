package redis

import "github.com/redis/go-redis/v9"

// NewClient creates a new Redis client with the given connection parameters.
func NewClient(addr, password string, db int) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
}

// Close gracefully shuts down the Redis client connection.
func Close(client *redis.Client) error {
	return client.Close()
}
