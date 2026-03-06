package redis_test

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redisinfra "github.com/truthmarket/truth-market/infra/redis"
	"github.com/truthmarket/truth-market/infra/testutil"
)

var testClient *goredis.Client

func TestMain(m *testing.M) {
	ctx := context.Background()

	connStr, cleanup, err := testutil.RedisContainer(ctx)
	if err != nil {
		panic("failed to start redis container: " + err.Error())
	}
	defer cleanup()

	// connStr is a redis:// URL; parse to extract host:port for go-redis.
	u, err := url.Parse(connStr)
	if err != nil {
		panic("failed to parse redis connection string: " + err.Error())
	}

	testClient = goredis.NewClient(&goredis.Options{
		Addr: u.Host,
	})

	// Verify connectivity.
	if err := testClient.Ping(ctx).Err(); err != nil {
		panic("failed to ping redis: " + err.Error())
	}

	code := m.Run()

	_ = testClient.Close()
	os.Exit(code)
}

// flushRedis removes all keys so each test starts with a clean store.
func flushRedis(t *testing.T) {
	t.Helper()
	require.NoError(t, testClient.FlushAll(context.Background()).Err())
}

// --------------------------------------------------------------------------
// SessionStore integration tests
// --------------------------------------------------------------------------

func TestSessionStore_Set_StoresWithTTL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	store := redisinfra.NewSessionStore(testClient)
	ctx := context.Background()

	token := "tok-abc-123"
	userID := "user-001"
	ttl := 10 * time.Second

	err := store.Set(ctx, token, userID, ttl)
	require.NoError(t, err)

	// Verify the key exists in Redis with a TTL.
	remaining := testClient.TTL(ctx, "session:"+token).Val()
	assert.True(t, remaining > 0 && remaining <= ttl,
		"expected TTL between 0 and %v, got %v", ttl, remaining)

	// Verify the stored value.
	val, err := testClient.Get(ctx, "session:"+token).Result()
	require.NoError(t, err)
	assert.Equal(t, userID, val)
}

func TestSessionStore_Get_ReturnsSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	store := redisinfra.NewSessionStore(testClient)
	ctx := context.Background()

	token := "tok-get-test"
	userID := "user-002"

	require.NoError(t, store.Set(ctx, token, userID, 30*time.Second))

	got, err := store.Get(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)
}

func TestSessionStore_Get_Expired_ReturnsEmpty(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	store := redisinfra.NewSessionStore(testClient)
	ctx := context.Background()

	token := "tok-expired"
	userID := "user-003"

	// Store with a very short TTL.
	require.NoError(t, store.Set(ctx, token, userID, 1*time.Millisecond))

	// Wait for the key to expire.
	time.Sleep(50 * time.Millisecond)

	_, err := store.Get(ctx, token)
	require.Error(t, err, "expected error for expired/missing session")
}

func TestSessionStore_Delete_RemovesSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	flushRedis(t)

	store := redisinfra.NewSessionStore(testClient)
	ctx := context.Background()

	token := "tok-delete-me"
	userID := "user-004"

	require.NoError(t, store.Set(ctx, token, userID, 5*time.Minute))

	// Confirm it exists first.
	got, err := store.Get(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, userID, got)

	// Delete.
	err = store.Delete(ctx, token)
	require.NoError(t, err)

	// Confirm it is gone.
	_, err = store.Get(ctx, token)
	require.Error(t, err, "expected error after deleting session")
}
