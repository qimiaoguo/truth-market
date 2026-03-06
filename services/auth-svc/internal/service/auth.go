package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/truthmarket/truth-market/pkg/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// AuthService handles authentication workflows including nonce generation,
// SIWE (Sign-In With Ethereum) verification, and user retrieval.
type AuthService struct {
	userRepo     repository.UserRepository
	sessionStore repository.SessionStore
	jwtSecret    string
}

// NewAuthService creates a new AuthService with the given dependencies.
func NewAuthService(userRepo repository.UserRepository, sessionStore repository.SessionStore, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:     userRepo,
		sessionStore: sessionStore,
		jwtSecret:    jwtSecret,
	}
}

// GenerateNonce generates a cryptographically random 32-byte hex nonce and
// stores it in the session store with a 5-minute TTL. The nonce can later be
// used to verify a SIWE message.
func (s *AuthService) GenerateNonce(ctx context.Context) (string, error) {
	// Generate 32 random bytes.
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	nonce := hex.EncodeToString(nonceBytes)

	// Store the nonce in the session store with key "nonce:<nonce>" and 5 min TTL.
	// We store "pending" as the value to indicate the nonce has been issued but
	// not yet consumed.
	nonceKey := fmt.Sprintf("nonce:%s", nonce)
	if err := s.sessionStore.Set(ctx, nonceKey, "pending", 5*time.Minute); err != nil {
		return "", fmt.Errorf("failed to store nonce: %w", err)
	}

	return nonce, nil
}

// VerifySIWE verifies a Sign-In With Ethereum message and signature.
// It extracts the nonce from the message, verifies it exists in the session
// store, checks if the user exists (creating a new one with 1000U balance if
// not), generates a JWT token, and returns the user and token.
//
// Phase 1 simplified verification: accepts any non-empty signature that does
// not end with "_bad". Real SIWE cryptographic verification will be added later.
func (s *AuthService) VerifySIWE(ctx context.Context, message, signature, walletAddress string) (*domain.User, string, error) {
	// Extract nonce from the message (look for "Nonce: " line).
	nonce, err := extractNonce(message)
	if err != nil {
		return nil, "", apperrors.Wrap(err, "UNAUTHORIZED", "invalid SIWE message: missing nonce")
	}

	// Verify the nonce exists in the session store.
	nonceKey := fmt.Sprintf("nonce:%s", nonce)
	_, err = s.sessionStore.Get(ctx, nonceKey)
	if err != nil {
		return nil, "", apperrors.New("UNAUTHORIZED", "invalid or expired nonce")
	}

	// Delete the nonce so it cannot be reused.
	_ = s.sessionStore.Delete(ctx, nonceKey)

	// Phase 1 simplified signature verification:
	// Accept any non-empty signature that does not end with "_bad".
	if signature == "" || strings.HasSuffix(signature, "_bad") {
		return nil, "", apperrors.New("UNAUTHORIZED", "invalid signature")
	}

	// Check if user already exists by wallet address.
	user, err := s.userRepo.GetByWallet(ctx, walletAddress)
	if err != nil {
		if !apperrors.IsNotFound(err) {
			return nil, "", fmt.Errorf("failed to look up user by wallet: %w", err)
		}

		// User does not exist -- create a new one with initial balance.
		userID, err := generateID()
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate user ID: %w", err)
		}

		user = &domain.User{
			ID:            userID,
			WalletAddress: walletAddress,
			UserType:      domain.UserTypeHuman,
			Balance:       decimal.InitialBalance,
			LockedBalance: decimal.Zero,
			CreatedAt:     time.Now(),
		}

		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, "", fmt.Errorf("failed to create user: %w", err)
		}
	}

	// Generate JWT token.
	token, err := s.generateJWT(user.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate JWT: %w", err)
	}

	return user, token, nil
}

// ValidateToken parses and validates a JWT token string. It verifies the
// HMAC-SHA256 signature, checks the expiration claim, extracts the subject
// (user ID), and returns the corresponding user from the repository.
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*domain.User, error) {
	// Split the token into its three parts: header.payload.signature.
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, apperrors.New("UNAUTHORIZED", "invalid token format")
	}

	header, payload, sig := parts[0], parts[1], parts[2]

	// Verify the HMAC-SHA256 signature.
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(signingInput))
	expectedSig := base64URLEncode(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return nil, apperrors.New("UNAUTHORIZED", "invalid token signature")
	}

	// Decode the payload.
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, apperrors.New("UNAUTHORIZED", "invalid token payload")
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, apperrors.New("UNAUTHORIZED", "invalid token claims")
	}

	// Check the expiration claim.
	expVal, ok := claims["exp"]
	if !ok {
		return nil, apperrors.New("UNAUTHORIZED", "token missing exp claim")
	}
	expFloat, ok := expVal.(float64)
	if !ok {
		return nil, apperrors.New("UNAUTHORIZED", "invalid exp claim")
	}
	if time.Now().Unix() > int64(expFloat) {
		return nil, apperrors.New("UNAUTHORIZED", "token expired")
	}

	// Extract the subject (user ID).
	subVal, ok := claims["sub"]
	if !ok {
		return nil, apperrors.New("UNAUTHORIZED", "token missing sub claim")
	}
	userID, ok := subVal.(string)
	if !ok || userID == "" {
		return nil, apperrors.New("UNAUTHORIZED", "invalid sub claim")
	}

	// Look up the user by ID.
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.New("UNAUTHORIZED", "user not found")
		}
		return nil, fmt.Errorf("failed to look up user: %w", err)
	}

	return user, nil
}

// GetUser retrieves a user by their ID from the repository.
func (s *AuthService) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// extractNonce parses a SIWE message to find the "Nonce: <value>" line.
func extractNonce(message string) (string, error) {
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Nonce: ") {
			nonce := strings.TrimPrefix(line, "Nonce: ")
			nonce = strings.TrimSpace(nonce)
			if nonce == "" {
				return "", fmt.Errorf("empty nonce in SIWE message")
			}
			return nonce, nil
		}
	}
	return "", fmt.Errorf("nonce not found in SIWE message")
}

// generateID creates a random hex-encoded identifier (16 bytes = 32 hex chars).
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// jwtHeader is the static JWT header for HS256.
var jwtHeader = base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))

// generateJWT creates a minimal JWT token encoding the user ID and expiration.
func (s *AuthService) generateJWT(userID string) (string, error) {
	payload := map[string]interface{}{
		"sub": userID,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JWT payload: %w", err)
	}

	encodedPayload := base64URLEncode(payloadBytes)

	// Create signature: HMAC-SHA256(header.payload, secret).
	signingInput := jwtHeader + "." + encodedPayload
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	mac.Write([]byte(signingInput))
	sig := base64URLEncode(mac.Sum(nil))

	return signingInput + "." + sig, nil
}

// base64URLEncode encodes bytes to base64url without padding (per JWT spec).
func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
