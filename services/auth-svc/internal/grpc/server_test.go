package grpc_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
	authgrpc "github.com/truthmarket/truth-market/services/auth-svc/internal/grpc"
)

// ---------------------------------------------------------------------------
// Mock services
// ---------------------------------------------------------------------------

// mockAuthService implements authgrpc.AuthServicer for testing.
type mockAuthService struct {
	generateNonceFn func(ctx context.Context) (string, error)
	verifySIWEFn    func(ctx context.Context, message, signature, walletAddress string) (*domain.User, string, error)
	validateTokenFn func(ctx context.Context, token string) (*domain.User, error)
	getUserFn       func(ctx context.Context, userID string) (*domain.User, error)
}

func (m *mockAuthService) GenerateNonce(ctx context.Context) (string, error) {
	return m.generateNonceFn(ctx)
}

func (m *mockAuthService) VerifySIWE(ctx context.Context, message, signature, walletAddress string) (*domain.User, string, error) {
	return m.verifySIWEFn(ctx, message, signature, walletAddress)
}

func (m *mockAuthService) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	return m.validateTokenFn(ctx, token)
}

func (m *mockAuthService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return m.getUserFn(ctx, userID)
}

// mockAPIKeyService implements authgrpc.APIKeyServicer for testing.
type mockAPIKeyService struct {
	generateAPIKeyFn func(ctx context.Context, userID, label string) (string, *domain.APIKey, error)
	validateAPIKeyFn func(ctx context.Context, rawKey string) (*domain.User, error)
	revokeAPIKeyFn   func(ctx context.Context, userID, keyPrefix string) error
}

func (m *mockAPIKeyService) GenerateAPIKey(ctx context.Context, userID, label string) (string, *domain.APIKey, error) {
	return m.generateAPIKeyFn(ctx, userID, label)
}

func (m *mockAPIKeyService) ValidateAPIKey(ctx context.Context, rawKey string) (*domain.User, error) {
	return m.validateAPIKeyFn(ctx, rawKey)
}

func (m *mockAPIKeyService) RevokeAPIKey(ctx context.Context, userID, keyPrefix string) error {
	return m.revokeAPIKeyFn(ctx, userID, keyPrefix)
}

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

const bufSize = 1024 * 1024

// testEnv bundles a bufconn-based gRPC client together with the mock services
// so each test can configure behaviour and make real gRPC calls.
type testEnv struct {
	client     authv1.AuthServiceClient
	authSvc    *mockAuthService
	apiKeySvc  *mockAPIKeyService
	conn       *grpc.ClientConn
	grpcServer *grpc.Server
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	authSvc := &mockAuthService{}
	apiKeySvc := &mockAPIKeyService{}

	srv := authgrpc.NewAuthServer(authSvc, apiKeySvc)

	lis := bufconn.Listen(bufSize)
	gs := grpc.NewServer()
	authv1.RegisterAuthServiceServer(gs, srv)

	go func() {
		if err := gs.Serve(lis); err != nil {
			// The server will return an error when we call GracefulStop; that
			// is expected and does not indicate a real failure.
		}
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}
	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		conn.Close()
		gs.GracefulStop()
	})

	return &testEnv{
		client:     authv1.NewAuthServiceClient(conn),
		authSvc:    authSvc,
		apiKeySvc:  apiKeySvc,
		conn:       conn,
		grpcServer: gs,
	}
}

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

func testUser() *domain.User {
	return &domain.User{
		ID:            "user-123",
		WalletAddress: "0xAbC123",
		UserType:      domain.UserTypeHuman,
		Balance:       decimal.NewFromFloat(100.50),
		LockedBalance: decimal.NewFromFloat(10.00),
		IsAdmin:       false,
		CreatedAt:     time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestGRPC_GenerateNonce_ReturnsNonce(t *testing.T) {
	env := newTestEnv(t)

	env.authSvc.generateNonceFn = func(_ context.Context) (string, error) {
		return "random-nonce-42", nil
	}

	resp, err := env.client.GenerateNonce(context.Background(), &authv1.GenerateNonceRequest{})
	require.NoError(t, err)
	assert.Equal(t, "random-nonce-42", resp.GetNonce())
}

func TestGRPC_VerifySignature_ValidPayload_ReturnsUserAndToken(t *testing.T) {
	env := newTestEnv(t)

	u := testUser()
	env.authSvc.verifySIWEFn = func(_ context.Context, message, signature, walletAddress string) (*domain.User, string, error) {
		assert.Equal(t, "siwe-message", message)
		assert.Equal(t, "0xsig", signature)
		assert.Equal(t, "0xAbC123", walletAddress)
		return u, "jwt-token-xyz", nil
	}

	resp, err := env.client.VerifySignature(context.Background(), &authv1.VerifySignatureRequest{
		Message:       "siwe-message",
		Signature:     "0xsig",
		WalletAddress: "0xAbC123",
	})
	require.NoError(t, err)
	assert.Equal(t, "jwt-token-xyz", resp.GetToken())
	assert.Equal(t, u.ID, resp.GetUser().GetId())
	assert.Equal(t, u.WalletAddress, resp.GetUser().GetWalletAddress())
	assert.Equal(t, authv1.UserType_USER_TYPE_HUMAN, resp.GetUser().GetUserType())
	assert.Equal(t, u.Balance.String(), resp.GetUser().GetBalance())
}

func TestGRPC_ValidateAPIKey_ValidKey_ReturnsUser(t *testing.T) {
	env := newTestEnv(t)

	u := testUser()
	env.apiKeySvc.validateAPIKeyFn = func(_ context.Context, rawKey string) (*domain.User, error) {
		assert.Equal(t, "tm_live_abc123", rawKey)
		return u, nil
	}

	resp, err := env.client.ValidateAPIKey(context.Background(), &authv1.ValidateAPIKeyRequest{
		ApiKey: "tm_live_abc123",
	})
	require.NoError(t, err)
	assert.Equal(t, u.ID, resp.GetUser().GetId())
	assert.Equal(t, u.WalletAddress, resp.GetUser().GetWalletAddress())
}

func TestGRPC_ValidateAPIKey_InvalidKey_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.apiKeySvc.validateAPIKeyFn = func(_ context.Context, _ string) (*domain.User, error) {
		return nil, apperrors.Wrap(fmt.Errorf("hash mismatch"), "UNAUTHORIZED", "invalid api key")
	}

	resp, err := env.client.ValidateAPIKey(context.Background(), &authv1.ValidateAPIKeyRequest{
		ApiKey: "bad-key",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "invalid api key", st.Message())
}

func TestGRPC_ValidateToken_ValidToken_ReturnsUser(t *testing.T) {
	env := newTestEnv(t)

	u := testUser()
	env.authSvc.validateTokenFn = func(_ context.Context, token string) (*domain.User, error) {
		assert.Equal(t, "valid-jwt-token", token)
		return u, nil
	}

	resp, err := env.client.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{
		Token: "valid-jwt-token",
	})
	require.NoError(t, err)
	assert.Equal(t, u.ID, resp.GetUser().GetId())
	assert.Equal(t, u.WalletAddress, resp.GetUser().GetWalletAddress())
	assert.Equal(t, authv1.UserType_USER_TYPE_HUMAN, resp.GetUser().GetUserType())
	assert.Equal(t, u.Balance.String(), resp.GetUser().GetBalance())
}

func TestGRPC_ValidateToken_InvalidToken_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.authSvc.validateTokenFn = func(_ context.Context, _ string) (*domain.User, error) {
		return nil, apperrors.New("UNAUTHORIZED", "invalid token signature")
	}

	resp, err := env.client.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{
		Token: "bad-token",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "invalid token signature", st.Message())
}

func TestGRPC_ValidateToken_ExpiredToken_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.authSvc.validateTokenFn = func(_ context.Context, _ string) (*domain.User, error) {
		return nil, apperrors.New("UNAUTHORIZED", "token expired")
	}

	resp, err := env.client.ValidateToken(context.Background(), &authv1.ValidateTokenRequest{
		Token: "expired-token",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Equal(t, "token expired", st.Message())
}

func TestGRPC_GetUser_ReturnsUser(t *testing.T) {
	env := newTestEnv(t)

	u := testUser()
	env.authSvc.getUserFn = func(_ context.Context, userID string) (*domain.User, error) {
		assert.Equal(t, "user-123", userID)
		return u, nil
	}

	resp, err := env.client.GetUser(context.Background(), &authv1.GetUserRequest{
		UserId: "user-123",
	})
	require.NoError(t, err)
	assert.Equal(t, u.ID, resp.GetUser().GetId())
	assert.Equal(t, u.WalletAddress, resp.GetUser().GetWalletAddress())
	assert.Equal(t, authv1.UserType_USER_TYPE_HUMAN, resp.GetUser().GetUserType())
	assert.Equal(t, u.Balance.String(), resp.GetUser().GetBalance())
	assert.Equal(t, u.LockedBalance.String(), resp.GetUser().GetLockedBalance())
	assert.Equal(t, u.IsAdmin, resp.GetUser().GetIsAdmin())
}

func TestGRPC_GetUser_NotFound_ReturnsError(t *testing.T) {
	env := newTestEnv(t)

	env.authSvc.getUserFn = func(_ context.Context, _ string) (*domain.User, error) {
		return nil, apperrors.Wrap(fmt.Errorf("no rows"), "NOT_FOUND", "user not found")
	}

	resp, err := env.client.GetUser(context.Background(), &authv1.GetUserRequest{
		UserId: "nonexistent",
	})
	assert.Nil(t, resp)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "user not found", st.Message())
}
