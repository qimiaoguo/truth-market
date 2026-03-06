// Package grpc implements the auth-svc gRPC transport layer.
//
// AuthServer adapts the AuthServicer and APIKeyServicer business-logic
// interfaces to the generated authv1.AuthServiceServer contract. Every method
// delegates to the underlying service, converts domain types to proto messages,
// and translates pkg/errors sentinels to the appropriate gRPC status codes.
package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	authv1 "github.com/truthmarket/truth-market/proto/gen/go/auth/v1"
)

// ---------------------------------------------------------------------------
// Service interfaces – the gRPC layer depends on these abstractions so that
// the concrete service implementations can be injected (and mocked in tests).
// ---------------------------------------------------------------------------

// AuthServicer defines the business operations for authentication.
type AuthServicer interface {
	GenerateNonce(ctx context.Context) (string, error)
	VerifySIWE(ctx context.Context, message, signature, walletAddress string) (*domain.User, string, error)
	ValidateToken(ctx context.Context, token string) (*domain.User, error)
	GetUser(ctx context.Context, userID string) (*domain.User, error)
}

// APIKeyServicer defines the business operations for API key management.
type APIKeyServicer interface {
	GenerateAPIKey(ctx context.Context, userID, label string) (string, *domain.APIKey, error)
	ValidateAPIKey(ctx context.Context, rawKey string) (*domain.User, error)
	RevokeAPIKey(ctx context.Context, userID, keyPrefix string) error
}

// ---------------------------------------------------------------------------
// AuthServer
// ---------------------------------------------------------------------------

// AuthServer implements authv1.AuthServiceServer by delegating to the
// AuthServicer and APIKeyServicer interfaces.
type AuthServer struct {
	authv1.UnimplementedAuthServiceServer
	authService   AuthServicer
	apiKeyService APIKeyServicer
}

// NewAuthServer constructs an AuthServer with the given service dependencies.
func NewAuthServer(auth AuthServicer, apiKey APIKeyServicer) *AuthServer {
	return &AuthServer{
		authService:   auth,
		apiKeyService: apiKey,
	}
}

// ---------------------------------------------------------------------------
// gRPC method implementations
// ---------------------------------------------------------------------------

// GenerateNonce returns a fresh nonce for SIWE authentication.
func (s *AuthServer) GenerateNonce(ctx context.Context, _ *authv1.GenerateNonceRequest) (*authv1.GenerateNonceResponse, error) {
	nonce, err := s.authService.GenerateNonce(ctx)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authv1.GenerateNonceResponse{Nonce: nonce}, nil
}

// VerifySignature verifies a SIWE signature and returns the authenticated
// user together with a session token.
func (s *AuthServer) VerifySignature(ctx context.Context, req *authv1.VerifySignatureRequest) (*authv1.VerifySignatureResponse, error) {
	user, token, err := s.authService.VerifySIWE(ctx, req.GetMessage(), req.GetSignature(), req.GetWalletAddress())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authv1.VerifySignatureResponse{
		User:  domainUserToProto(user),
		Token: token,
	}, nil
}

// ValidateToken validates a JWT/session token and returns the associated user.
func (s *AuthServer) ValidateToken(ctx context.Context, req *authv1.ValidateTokenRequest) (*authv1.ValidateTokenResponse, error) {
	user, err := s.authService.ValidateToken(ctx, req.GetToken())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authv1.ValidateTokenResponse{
		User: domainUserToProto(user),
	}, nil
}

// ValidateAPIKey validates a raw API key and returns the owning user.
func (s *AuthServer) ValidateAPIKey(ctx context.Context, req *authv1.ValidateAPIKeyRequest) (*authv1.ValidateAPIKeyResponse, error) {
	user, err := s.apiKeyService.ValidateAPIKey(ctx, req.GetApiKey())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authv1.ValidateAPIKeyResponse{
		User: domainUserToProto(user),
	}, nil
}

// CreateAPIKey generates a new API key for the specified user.
func (s *AuthServer) CreateAPIKey(ctx context.Context, req *authv1.CreateAPIKeyRequest) (*authv1.CreateAPIKeyResponse, error) {
	rawKey, apiKey, err := s.apiKeyService.GenerateAPIKey(ctx, req.GetUserId(), req.GetLabel())
	if err != nil {
		return nil, toGRPCError(err)
	}

	resp := &authv1.CreateAPIKeyResponse{
		Key:       rawKey,
		KeyPrefix: apiKey.KeyPrefix,
	}
	if apiKey.ExpiresAt != nil {
		resp.ExpiresAt = timestamppb.New(*apiKey.ExpiresAt)
	}
	return resp, nil
}

// RevokeAPIKey revokes an existing API key identified by its prefix.
func (s *AuthServer) RevokeAPIKey(ctx context.Context, req *authv1.RevokeAPIKeyRequest) (*authv1.RevokeAPIKeyResponse, error) {
	if err := s.apiKeyService.RevokeAPIKey(ctx, req.GetUserId(), req.GetKeyPrefix()); err != nil {
		return nil, toGRPCError(err)
	}
	return &authv1.RevokeAPIKeyResponse{}, nil
}

// GetUser retrieves a user by ID.
func (s *AuthServer) GetUser(ctx context.Context, req *authv1.GetUserRequest) (*authv1.GetUserResponse, error) {
	user, err := s.authService.GetUser(ctx, req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &authv1.GetUserResponse{
		User: domainUserToProto(user),
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// domainUserToProto converts a domain.User to the protobuf User message.
func domainUserToProto(u *domain.User) *authv1.User {
	if u == nil {
		return nil
	}
	pb := &authv1.User{
		Id:            u.ID,
		WalletAddress: u.WalletAddress,
		Balance:       u.Balance.String(),
		LockedBalance: u.LockedBalance.String(),
		IsAdmin:       u.IsAdmin,
		CreatedAt:     timestamppb.New(u.CreatedAt),
	}
	switch u.UserType {
	case domain.UserTypeHuman:
		pb.UserType = authv1.UserType_USER_TYPE_HUMAN
	case domain.UserTypeAgent:
		pb.UserType = authv1.UserType_USER_TYPE_AGENT
	default:
		pb.UserType = authv1.UserType_USER_TYPE_UNSPECIFIED
	}
	return pb
}

// toGRPCError translates an application error (pkg/errors.AppError) to the
// corresponding gRPC status error. Unknown error codes fall through to
// codes.Internal.
func toGRPCError(err error) error {
	if err == nil {
		return nil
	}

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		return status.Error(codes.Internal, err.Error())
	}

	switch appErr.Code {
	case apperrors.ErrNotFound.Code:
		return status.Error(codes.NotFound, appErr.Message)
	case apperrors.ErrUnauthorized.Code:
		return status.Error(codes.Unauthenticated, appErr.Message)
	case apperrors.ErrForbidden.Code:
		return status.Error(codes.PermissionDenied, appErr.Message)
	case apperrors.ErrBadRequest.Code:
		return status.Error(codes.InvalidArgument, appErr.Message)
	case apperrors.ErrConflict.Code:
		return status.Error(codes.AlreadyExists, appErr.Message)
	case apperrors.ErrInternalError.Code:
		return status.Error(codes.Internal, appErr.Message)
	case apperrors.ErrInsufficientBalance.Code:
		return status.Error(codes.FailedPrecondition, appErr.Message)
	case apperrors.ErrMarketClosed.Code:
		return status.Error(codes.FailedPrecondition, appErr.Message)
	case apperrors.ErrInvalidPrice.Code:
		return status.Error(codes.InvalidArgument, appErr.Message)
	default:
		return status.Error(codes.Internal, appErr.Message)
	}
}
