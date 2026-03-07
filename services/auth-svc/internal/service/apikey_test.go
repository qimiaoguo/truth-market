package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mock: APIKeyRepository
// ---------------------------------------------------------------------------

type mockAPIKeyRepo struct {
	keys     map[string]*domain.APIKey // keyed by ID
	byHash   map[string]*domain.APIKey // keyed by KeyHash
	byUser   map[string][]*domain.APIKey
	createFn func(ctx context.Context, key *domain.APIKey) error
	revokeFn func(ctx context.Context, id string) error
}

func newMockAPIKeyRepo() *mockAPIKeyRepo {
	return &mockAPIKeyRepo{
		keys:   make(map[string]*domain.APIKey),
		byHash: make(map[string]*domain.APIKey),
		byUser: make(map[string][]*domain.APIKey),
	}
}

func (m *mockAPIKeyRepo) Create(ctx context.Context, key *domain.APIKey) error {
	if m.createFn != nil {
		return m.createFn(ctx, key)
	}
	m.keys[key.ID] = key
	m.byHash[key.KeyHash] = key
	m.byUser[key.UserID] = append(m.byUser[key.UserID], key)
	return nil
}

func (m *mockAPIKeyRepo) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	k, ok := m.byHash[hash]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return k, nil
}

func (m *mockAPIKeyRepo) ListByUser(ctx context.Context, userID string) ([]*domain.APIKey, error) {
	keys := m.byUser[userID]
	if keys == nil {
		return []*domain.APIKey{}, nil
	}
	return keys, nil
}

func (m *mockAPIKeyRepo) Revoke(ctx context.Context, id string) error {
	if m.revokeFn != nil {
		return m.revokeFn(ctx, id)
	}
	k, ok := m.keys[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	k.IsActive = false
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestAPIKeyService creates an APIKeyService wired with in-memory mocks.
func newTestAPIKeyService() (*APIKeyService, *mockAPIKeyRepo, *mockUserRepo) {
	apiKeyRepo := newMockAPIKeyRepo()
	userRepo := newMockUserRepo()
	svc := NewAPIKeyService(apiKeyRepo, userRepo)
	return svc, apiKeyRepo, userRepo
}

// seedUserForAPIKey adds a user to the mock repo for API key tests.
func seedUserForAPIKey(repo *mockUserRepo, id, wallet string) *domain.User {
	u := &domain.User{
		ID:            id,
		WalletAddress: wallet,
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(1000),
		LockedBalance: decimal.NewFromInt(0),
		CreatedAt:     time.Now(),
	}
	repo.users[u.ID] = u
	repo.byWallet[strings.ToLower(u.WalletAddress)] = u
	return u
}

// hashKey computes the SHA-256 hex digest of a raw API key for assertions.
func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Tests: GenerateAPIKey
// ---------------------------------------------------------------------------

func TestGenerateAPIKey_ReturnsKeyWithPrefix(t *testing.T) {
	svc, _, userRepo := newTestAPIKeyService()
	ctx := context.Background()

	user := seedUserForAPIKey(userRepo, "user-1", "0x1111111111111111111111111111111111111111")

	rawKey, apiKey, err := svc.GenerateAPIKey(ctx, user.ID, "My Test Key")
	require.NoError(t, err)

	// Raw key must start with "tm_" prefix.
	assert.True(t, strings.HasPrefix(rawKey, "tm_"),
		"raw key should start with 'tm_' prefix, got: %s", rawKey)

	// Raw key should have sufficient length (prefix + random bytes).
	assert.GreaterOrEqual(t, len(rawKey), 20,
		"raw key should be at least 20 characters long")

	// APIKey metadata should be populated.
	assert.NotEmpty(t, apiKey.ID, "API key should have an ID")
	assert.Equal(t, user.ID, apiKey.UserID, "API key should belong to the user")
	assert.True(t, apiKey.IsActive, "new API key should be active")
	assert.NotEmpty(t, apiKey.KeyPrefix, "API key should have a prefix for identification")

	// The KeyPrefix stored should match the beginning of the raw key.
	assert.True(t, strings.HasPrefix(rawKey, apiKey.KeyPrefix),
		"raw key should start with the stored KeyPrefix")

	// KeyPrefix must fit in the DB column (VARCHAR(10)).
	assert.LessOrEqual(t, len(apiKey.KeyPrefix), 10,
		"key prefix must be at most 10 chars to fit DB column, got %d: %q",
		len(apiKey.KeyPrefix), apiKey.KeyPrefix)
}

func TestGenerateAPIKey_StoresOnlyHash(t *testing.T) {
	svc, apiKeyRepo, userRepo := newTestAPIKeyService()
	ctx := context.Background()

	user := seedUserForAPIKey(userRepo, "user-2", "0x2222222222222222222222222222222222222222")

	rawKey, apiKey, err := svc.GenerateAPIKey(ctx, user.ID, "Hash Test Key")
	require.NoError(t, err)

	// The stored hash must NOT equal the raw key (it should be a SHA-256 digest).
	assert.NotEqual(t, rawKey, apiKey.KeyHash,
		"stored hash should not be the raw key")

	// The stored hash should be the SHA-256 of the raw key.
	expectedHash := hashKey(rawKey)
	assert.Equal(t, expectedHash, apiKey.KeyHash,
		"stored hash should be SHA-256 of the raw key")

	// Verify the key was stored in the repository with the hash.
	stored, err := apiKeyRepo.GetByHash(ctx, expectedHash)
	require.NoError(t, err, "key should be retrievable by its hash")
	assert.Equal(t, apiKey.ID, stored.ID, "stored key ID should match")
}

// ---------------------------------------------------------------------------
// Tests: ValidateAPIKey
// ---------------------------------------------------------------------------

func TestValidateAPIKey_ValidKey_ReturnsUser(t *testing.T) {
	svc, _, userRepo := newTestAPIKeyService()
	ctx := context.Background()

	user := seedUserForAPIKey(userRepo, "user-3", "0x3333333333333333333333333333333333333333")

	// Generate a key, then validate it.
	rawKey, _, err := svc.GenerateAPIKey(ctx, user.ID, "Validate Test Key")
	require.NoError(t, err)

	validatedUser, err := svc.ValidateAPIKey(ctx, rawKey)
	require.NoError(t, err)

	assert.Equal(t, user.ID, validatedUser.ID,
		"validated user ID should match the key owner")
	assert.Equal(t, user.WalletAddress, validatedUser.WalletAddress,
		"validated user wallet should match")
}

func TestValidateAPIKey_InvalidKey_ReturnsError(t *testing.T) {
	svc, _, _ := newTestAPIKeyService()
	ctx := context.Background()

	user, err := svc.ValidateAPIKey(ctx, "tm_this_key_does_not_exist_anywhere")

	assert.Error(t, err, "invalid key should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED for invalid key, got: %v", err)
	assert.Nil(t, user, "no user should be returned for invalid key")
}

func TestValidateAPIKey_RevokedKey_ReturnsError(t *testing.T) {
	svc, apiKeyRepo, userRepo := newTestAPIKeyService()
	ctx := context.Background()

	user := seedUserForAPIKey(userRepo, "user-4", "0x4444444444444444444444444444444444444444")

	// Generate and then revoke the key.
	rawKey, apiKey, err := svc.GenerateAPIKey(ctx, user.ID, "Revoke Validate Test")
	require.NoError(t, err)

	// Revoke via the repo directly (simulate revocation).
	err = apiKeyRepo.Revoke(ctx, apiKey.ID)
	require.NoError(t, err)

	// Now validation should fail.
	validatedUser, err := svc.ValidateAPIKey(ctx, rawKey)

	assert.Error(t, err, "revoked key should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED for revoked key, got: %v", err)
	assert.Nil(t, validatedUser, "no user should be returned for revoked key")
}

// ---------------------------------------------------------------------------
// Tests: RevokeAPIKey
// ---------------------------------------------------------------------------

func TestRevokeAPIKey_DeactivatesKey(t *testing.T) {
	svc, apiKeyRepo, userRepo := newTestAPIKeyService()
	ctx := context.Background()

	user := seedUserForAPIKey(userRepo, "user-5", "0x5555555555555555555555555555555555555555")

	_, apiKey, err := svc.GenerateAPIKey(ctx, user.ID, "Revoke Test Key")
	require.NoError(t, err)

	// Revoke using the service method, passing the key prefix for identification.
	err = svc.RevokeAPIKey(ctx, user.ID, apiKey.KeyPrefix)
	require.NoError(t, err)

	// Verify the key is now inactive in the repository.
	stored := apiKeyRepo.keys[apiKey.ID]
	require.NotNil(t, stored, "key should still exist in repo after revocation")
	assert.False(t, stored.IsActive, "revoked key should be inactive")
}

func TestRevokeAPIKey_NotOwner_ReturnsError(t *testing.T) {
	svc, _, userRepo := newTestAPIKeyService()
	ctx := context.Background()

	owner := seedUserForAPIKey(userRepo, "owner-user", "0xAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	otherUser := seedUserForAPIKey(userRepo, "other-user", "0xBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB")

	_, apiKey, err := svc.GenerateAPIKey(ctx, owner.ID, "Owner's Key")
	require.NoError(t, err)

	// A different user tries to revoke the key.
	err = svc.RevokeAPIKey(ctx, otherUser.ID, apiKey.KeyPrefix)

	assert.Error(t, err, "non-owner should not be able to revoke the key")
	assert.True(t, apperrors.IsForbidden(err),
		"error should be FORBIDDEN when non-owner tries to revoke, got: %v", err)
}
