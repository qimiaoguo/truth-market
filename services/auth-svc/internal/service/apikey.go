package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// APIKeyService handles API key generation, validation, and revocation.
type APIKeyService struct {
	apiKeyRepo repository.APIKeyRepository
	userRepo   repository.UserRepository
}

// NewAPIKeyService creates a new APIKeyService with the given dependencies.
func NewAPIKeyService(apiKeyRepo repository.APIKeyRepository, userRepo repository.UserRepository) *APIKeyService {
	return &APIKeyService{
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
	}
}

// GenerateAPIKey creates a new API key for the given user. It generates a
// random key with a "tm_" prefix (24 random hex bytes = 48 chars after prefix),
// stores only the SHA-256 hash in the repository, and returns the raw key along
// with the APIKey metadata.
func (s *APIKeyService) GenerateAPIKey(ctx context.Context, userID, label string) (string, *domain.APIKey, error) {
	// Generate 24 random bytes for the key body.
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random key: %w", err)
	}

	rawKey := "tm_" + hex.EncodeToString(randomBytes)

	// Compute SHA-256 hash of the raw key.
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Generate an ID for the API key record.
	keyID, err := generateID()
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate API key ID: %w", err)
	}

	// Store a prefix for display/identification purposes (e.g., "tm_abcdef12").
	// We use the first 12 characters of the raw key as the prefix.
	keyPrefix := rawKey[:12]

	apiKey := &domain.APIKey{
		ID:        keyID,
		UserID:    userID,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if err := s.apiKeyRepo.Create(ctx, apiKey); err != nil {
		return "", nil, fmt.Errorf("failed to store API key: %w", err)
	}

	return rawKey, apiKey, nil
}

// ValidateAPIKey takes a raw API key, computes its SHA-256 hash, looks it up
// in the repository, verifies it is active, and returns the associated user.
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, rawKey string) (*domain.User, error) {
	// Compute SHA-256 hash of the raw key.
	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])

	// Look up the key by its hash.
	apiKey, err := s.apiKeyRepo.GetByHash(ctx, keyHash)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return nil, apperrors.New("UNAUTHORIZED", "invalid API key")
		}
		return nil, fmt.Errorf("failed to look up API key: %w", err)
	}

	// Check that the key is still active.
	if !apiKey.IsActive {
		return nil, apperrors.New("UNAUTHORIZED", "API key has been revoked")
	}

	// Return the associated user.
	user, err := s.userRepo.GetByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up user for API key: %w", err)
	}

	return user, nil
}

// RevokeAPIKey verifies that the calling user owns the API key identified by
// keyPrefix, then deactivates it.
func (s *APIKeyService) RevokeAPIKey(ctx context.Context, userID, keyPrefix string) error {
	// List all keys belonging to the user to find the one matching the prefix.
	keys, err := s.apiKeyRepo.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to list user API keys: %w", err)
	}

	// Find the key matching the prefix.
	var targetKey *domain.APIKey
	for _, k := range keys {
		if k.KeyPrefix == keyPrefix {
			targetKey = k
			break
		}
	}

	if targetKey == nil {
		// The key prefix doesn't belong to this user. It might belong to
		// another user or not exist at all. Either way, deny the operation.
		return apperrors.New("FORBIDDEN", "API key not found or not owned by user")
	}

	// Revoke the key.
	if err := s.apiKeyRepo.Revoke(ctx, targetKey.ID); err != nil {
		return fmt.Errorf("failed to revoke API key: %w", err)
	}

	return nil
}
