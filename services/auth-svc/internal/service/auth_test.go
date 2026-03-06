package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// ---------------------------------------------------------------------------
// Mock: UserRepository
// ---------------------------------------------------------------------------

type mockUserRepo struct {
	users    map[string]*domain.User
	byWallet map[string]*domain.User
	createFn func(ctx context.Context, user *domain.User) error
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:    make(map[string]*domain.User),
		byWallet: make(map[string]*domain.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	m.users[user.ID] = user
	m.byWallet[strings.ToLower(user.WalletAddress)] = user
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetByWallet(ctx context.Context, addr string) (*domain.User, error) {
	u, ok := m.byWallet[strings.ToLower(addr)]
	if !ok {
		return nil, apperrors.ErrNotFound
	}
	return u, nil
}

func (m *mockUserRepo) UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error {
	u, ok := m.users[id]
	if !ok {
		return apperrors.ErrNotFound
	}
	u.Balance = balance
	u.LockedBalance = locked
	return nil
}

func (m *mockUserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	all := make([]*domain.User, 0, len(m.users))
	for _, u := range m.users {
		all = append(all, u)
	}
	return all, int64(len(all)), nil
}

// ---------------------------------------------------------------------------
// Mock: SessionStore
// ---------------------------------------------------------------------------

type sessionEntry struct {
	userID string
	expiry time.Time
}

type mockSessionStore struct {
	sessions map[string]sessionEntry
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]sessionEntry),
	}
}

func (m *mockSessionStore) Set(ctx context.Context, token string, userID string, ttl time.Duration) error {
	m.sessions[token] = sessionEntry{
		userID: userID,
		expiry: time.Now().Add(ttl),
	}
	return nil
}

func (m *mockSessionStore) Get(ctx context.Context, token string) (string, error) {
	entry, ok := m.sessions[token]
	if !ok {
		return "", apperrors.ErrNotFound
	}
	if time.Now().After(entry.expiry) {
		delete(m.sessions, token)
		return "", apperrors.ErrNotFound
	}
	return entry.userID, nil
}

func (m *mockSessionStore) Delete(ctx context.Context, token string) error {
	delete(m.sessions, token)
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestAuthService creates an AuthService wired with in-memory mocks.
func newTestAuthService() (*AuthService, *mockUserRepo, *mockSessionStore) {
	userRepo := newMockUserRepo()
	sessionStore := newMockSessionStore()

	svc := NewAuthService(userRepo, sessionStore, "test-jwt-secret")
	return svc, userRepo, sessionStore
}

// seedUser inserts a user directly into the mock repo so tests can reference
// an "existing" user without going through the service.
func seedUser(repo *mockUserRepo, user *domain.User) {
	repo.users[user.ID] = user
	repo.byWallet[strings.ToLower(user.WalletAddress)] = user
}

// ---------------------------------------------------------------------------
// Tests: GenerateNonce
// ---------------------------------------------------------------------------

func TestGenerateNonce_ReturnsValidNonce(t *testing.T) {
	svc, _, sessionStore := newTestAuthService()
	ctx := context.Background()

	nonce, err := svc.GenerateNonce(ctx)
	require.NoError(t, err)

	// Nonce must be a non-empty string.
	assert.NotEmpty(t, nonce, "nonce should not be empty")

	// The nonce should be at least 16 characters (sufficient randomness).
	assert.GreaterOrEqual(t, len(nonce), 16, "nonce should be at least 16 characters")

	// Nonce should be stored in the session store so it can be validated later.
	storedValue, err := sessionStore.Get(ctx, fmt.Sprintf("nonce:%s", nonce))
	require.NoError(t, err, "nonce should be stored in session store")
	assert.NotEmpty(t, storedValue, "stored nonce value should not be empty")

	// Generating a second nonce should produce a different value.
	nonce2, err := svc.GenerateNonce(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, nonce, nonce2, "consecutive nonces should be unique")
}

// ---------------------------------------------------------------------------
// Tests: VerifySIWE
// ---------------------------------------------------------------------------

func TestVerifySIWE_ValidSignature_CreatesNewUser(t *testing.T) {
	svc, userRepo, _ := newTestAuthService()
	ctx := context.Background()

	walletAddr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	// The service needs to verify the SIWE signature. For unit tests the
	// service should accept a pluggable verifier or we test that the flow
	// calls the right dependencies. We simulate a valid verification by
	// pre-storing the nonce in the session store so the service can find it.
	nonce, err := svc.GenerateNonce(ctx)
	require.NoError(t, err)

	// Build a minimal SIWE message referencing the nonce.
	siweMessage := fmt.Sprintf("truth-market wants you to sign in with your Ethereum account:\n%s\n\nNonce: %s", walletAddr, nonce)
	// In Red-phase tests we pass a placeholder signature; the implementation
	// will handle real crypto verification.
	signature := "0xvalidsignature"

	user, token, err := svc.VerifySIWE(ctx, siweMessage, signature, walletAddr)
	require.NoError(t, err)

	// First login should create a new user.
	assert.NotNil(t, user, "user should be returned")
	assert.Equal(t, walletAddr, user.WalletAddress, "wallet address should match")
	assert.Equal(t, domain.UserTypeHuman, user.UserType, "user type should be human")

	// First-login balance: 1000 units.
	expectedBalance := decimal.NewFromInt(1000)
	assert.True(t, user.Balance.Equal(expectedBalance),
		"new user should receive 1000U initial balance, got %s", user.Balance.String())

	// A JWT token should be returned.
	assert.NotEmpty(t, token, "JWT token should not be empty")

	// The user should now be persisted in the repository.
	persisted, err := userRepo.GetByWallet(ctx, walletAddr)
	require.NoError(t, err)
	assert.Equal(t, user.ID, persisted.ID, "persisted user ID should match")
}

func TestVerifySIWE_ExistingUser_NoDoubleGrant(t *testing.T) {
	svc, userRepo, _ := newTestAuthService()
	ctx := context.Background()

	walletAddr := "0xExistingUser1234567890abcdef12345678901234"

	// Seed an existing user with an arbitrary balance.
	existingBalance := decimal.NewFromInt(500)
	existingUser := &domain.User{
		ID:            "existing-user-id",
		WalletAddress: walletAddr,
		UserType:      domain.UserTypeHuman,
		Balance:       existingBalance,
		LockedBalance: decimal.NewFromInt(0),
		CreatedAt:     time.Now().Add(-24 * time.Hour),
	}
	seedUser(userRepo, existingUser)

	// Generate nonce and build SIWE message.
	nonce, err := svc.GenerateNonce(ctx)
	require.NoError(t, err)
	siweMessage := fmt.Sprintf("truth-market wants you to sign in with your Ethereum account:\n%s\n\nNonce: %s", walletAddr, nonce)
	signature := "0xvalidsignature"

	user, token, err := svc.VerifySIWE(ctx, siweMessage, signature, walletAddr)
	require.NoError(t, err)

	// Should return the existing user.
	assert.Equal(t, existingUser.ID, user.ID, "should return existing user")

	// Balance should NOT change -- no double-grant of initial 1000U.
	assert.True(t, user.Balance.Equal(existingBalance),
		"existing user balance should not change, got %s", user.Balance.String())

	// Still returns a valid JWT.
	assert.NotEmpty(t, token, "JWT token should not be empty")
}

func TestVerifySIWE_InvalidSignature_ReturnsError(t *testing.T) {
	svc, _, _ := newTestAuthService()
	ctx := context.Background()

	walletAddr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	nonce, err := svc.GenerateNonce(ctx)
	require.NoError(t, err)

	siweMessage := fmt.Sprintf("truth-market wants you to sign in with your Ethereum account:\n%s\n\nNonce: %s", walletAddr, nonce)
	invalidSignature := "0xinvalidsignature_bad"

	user, token, err := svc.VerifySIWE(ctx, siweMessage, invalidSignature, walletAddr)

	assert.Error(t, err, "invalid signature should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED, got: %v", err)
	assert.Nil(t, user, "no user should be returned on invalid signature")
	assert.Empty(t, token, "no token should be returned on invalid signature")
}

func TestVerifySIWE_ExpiredNonce_ReturnsError(t *testing.T) {
	svc, _, sessionStore := newTestAuthService()
	ctx := context.Background()

	walletAddr := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"

	// Generate a nonce and then expire it by manipulating the mock store.
	nonce, err := svc.GenerateNonce(ctx)
	require.NoError(t, err)

	// Simulate expiration by deleting the nonce from the session store.
	nonceKey := fmt.Sprintf("nonce:%s", nonce)
	err = sessionStore.Delete(ctx, nonceKey)
	require.NoError(t, err)

	siweMessage := fmt.Sprintf("truth-market wants you to sign in with your Ethereum account:\n%s\n\nNonce: %s", walletAddr, nonce)
	signature := "0xvalidsignature"

	user, token, err := svc.VerifySIWE(ctx, siweMessage, signature, walletAddr)

	assert.Error(t, err, "expired nonce should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED for expired nonce, got: %v", err)
	assert.Nil(t, user, "no user should be returned with expired nonce")
	assert.Empty(t, token, "no token should be returned with expired nonce")
}

// ---------------------------------------------------------------------------
// Tests: GetUser
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Tests: ValidateToken
// ---------------------------------------------------------------------------

func TestValidateToken_ValidToken_ReturnsUser(t *testing.T) {
	svc, userRepo, _ := newTestAuthService()
	ctx := context.Background()

	// Seed a user.
	expected := &domain.User{
		ID:            "user-456",
		WalletAddress: "0xABCDEF1234567890abcdef1234567890ABCDEF12",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(500),
		LockedBalance: decimal.NewFromInt(25),
		CreatedAt:     time.Now(),
	}
	seedUser(userRepo, expected)

	// Generate a JWT for the user via the service (uses the same secret).
	token, err := svc.generateJWT(expected.ID)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// Validate the token.
	user, err := svc.ValidateToken(ctx, token)
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.Equal(t, expected.ID, user.ID)
	assert.Equal(t, expected.WalletAddress, user.WalletAddress)
	assert.True(t, expected.Balance.Equal(user.Balance), "balance should match")
}

func TestValidateToken_ExpiredToken_ReturnsError(t *testing.T) {
	svc, userRepo, _ := newTestAuthService()
	ctx := context.Background()

	// Seed a user.
	user := &domain.User{
		ID:            "user-789",
		WalletAddress: "0x1234567890abcdef1234567890abcdef12345678",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.NewFromInt(0),
		CreatedAt:     time.Now(),
	}
	seedUser(userRepo, user)

	// Build an expired token manually using the same JWT construction as
	// generateJWT but with an expiration in the past.
	token := buildTestJWT(t, user.ID, time.Now().Add(-1*time.Hour).Unix(), "test-jwt-secret")

	result, err := svc.ValidateToken(ctx, token)
	assert.Error(t, err, "expired token should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED, got: %v", err)
	assert.Nil(t, result, "no user should be returned for expired token")
}

func TestValidateToken_InvalidSignature_ReturnsError(t *testing.T) {
	svc, userRepo, _ := newTestAuthService()
	ctx := context.Background()

	// Seed a user.
	user := &domain.User{
		ID:            "user-789",
		WalletAddress: "0x1234567890abcdef1234567890abcdef12345678",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(100),
		LockedBalance: decimal.NewFromInt(0),
		CreatedAt:     time.Now(),
	}
	seedUser(userRepo, user)

	// Build a token signed with a DIFFERENT secret.
	token := buildTestJWT(t, user.ID, time.Now().Add(24*time.Hour).Unix(), "wrong-secret")

	result, err := svc.ValidateToken(ctx, token)
	assert.Error(t, err, "tampered token should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED, got: %v", err)
	assert.Nil(t, result, "no user should be returned for tampered token")
}

func TestValidateToken_MalformedToken_ReturnsError(t *testing.T) {
	svc, _, _ := newTestAuthService()
	ctx := context.Background()

	result, err := svc.ValidateToken(ctx, "not-a-jwt")
	assert.Error(t, err, "malformed token should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED, got: %v", err)
	assert.Nil(t, result, "no user should be returned for malformed token")
}

func TestValidateToken_UserNotFound_ReturnsError(t *testing.T) {
	svc, _, _ := newTestAuthService()
	ctx := context.Background()

	// Generate a valid token for a user ID that does not exist in the repo.
	token := buildTestJWT(t, "nonexistent-user-id", time.Now().Add(24*time.Hour).Unix(), "test-jwt-secret")

	result, err := svc.ValidateToken(ctx, token)
	assert.Error(t, err, "token for missing user should produce an error")
	assert.True(t, apperrors.IsUnauthorized(err),
		"error should be UNAUTHORIZED, got: %v", err)
	assert.Nil(t, result, "no user should be returned when user not found")
}

// buildTestJWT constructs a JWT token with the given subject, expiration, and
// secret. This mirrors the hand-rolled JWT logic in generateJWT so tests can
// produce tokens with custom claims.
func buildTestJWT(t *testing.T, sub string, exp int64, secret string) string {
	t.Helper()

	header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

	payload := fmt.Sprintf(`{"sub":"%s","exp":%d,"iat":%d}`, sub, exp, time.Now().Unix())
	encodedPayload := base64URLEncode([]byte(payload))

	signingInput := header + "." + encodedPayload

	mac := hmacSHA256([]byte(signingInput), []byte(secret))
	sig := base64URLEncode(mac)

	return signingInput + "." + sig
}

// hmacSHA256 returns the HMAC-SHA256 of data using the given key.
func hmacSHA256(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// ---------------------------------------------------------------------------
// Tests: GetUser
// ---------------------------------------------------------------------------

func TestGetUser_ReturnsUser(t *testing.T) {
	svc, userRepo, _ := newTestAuthService()
	ctx := context.Background()

	expected := &domain.User{
		ID:            "user-123",
		WalletAddress: "0xABCDEF1234567890abcdef1234567890ABCDEF12",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromInt(750),
		LockedBalance: decimal.NewFromInt(50),
		CreatedAt:     time.Now(),
	}
	seedUser(userRepo, expected)

	user, err := svc.GetUser(ctx, "user-123")
	require.NoError(t, err)

	assert.Equal(t, expected.ID, user.ID)
	assert.Equal(t, expected.WalletAddress, user.WalletAddress)
	assert.True(t, expected.Balance.Equal(user.Balance), "balance should match")
}

func TestGetUser_NotFound_ReturnsError(t *testing.T) {
	svc, _, _ := newTestAuthService()
	ctx := context.Background()

	user, err := svc.GetUser(ctx, "nonexistent-user-id")

	assert.Error(t, err, "should return an error for missing user")
	assert.True(t, apperrors.IsNotFound(err),
		"error should be NOT_FOUND, got: %v", err)
	assert.Nil(t, user, "no user should be returned when not found")
}
