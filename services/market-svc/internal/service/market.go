package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// CreateMarketRequest holds the input data required to create a new prediction market.
type CreateMarketRequest struct {
	Title         string
	Description   string
	MarketType    domain.MarketType
	Category      string
	OutcomeLabels []string
	EndTime       time.Time
	CreatedBy     string
}

// MarketService implements the core business logic for prediction markets.
type MarketService struct {
	marketRepo  repository.MarketRepository
	outcomeRepo repository.OutcomeRepository
	txManager   repository.TxManager
}

// NewMarketService constructs a MarketService with the given dependencies.
func NewMarketService(
	marketRepo repository.MarketRepository,
	outcomeRepo repository.OutcomeRepository,
	txManager repository.TxManager,
) *MarketService {
	return &MarketService{
		marketRepo:  marketRepo,
		outcomeRepo: outcomeRepo,
		txManager:   txManager,
	}
}

// generateID produces a random hex-encoded identifier.
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand unavailable: %v", err))
	}
	return hex.EncodeToString(b)
}

// validTransitions defines the allowed state machine transitions for a market.
var validTransitions = map[domain.MarketStatus][]domain.MarketStatus{
	domain.MarketStatusDraft:  {domain.MarketStatusOpen},
	domain.MarketStatusOpen:   {domain.MarketStatusClosed},
	domain.MarketStatusClosed: {domain.MarketStatusResolved, domain.MarketStatusCancelled},
	// resolved and cancelled are terminal -- no outgoing transitions.
}

// isValidTransition checks whether moving from one status to another is allowed.
func isValidTransition(from, to domain.MarketStatus) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// CreateMarket validates the request, then atomically creates a market and its
// outcomes inside a transaction. Binary markets always receive "Yes"/"No"
// outcomes; multi markets use the supplied labels.
func (s *MarketService) CreateMarket(ctx context.Context, req CreateMarketRequest) (*domain.Market, error) {
	// --- validation ---
	if req.Title == "" {
		return nil, apperrors.New("BAD_REQUEST", "title is required")
	}

	if req.MarketType == domain.MarketTypeBinary {
		// Binary markets must have either 0 or exactly 2 outcome labels.
		if len(req.OutcomeLabels) != 0 && len(req.OutcomeLabels) != 2 {
			return nil, apperrors.New("BAD_REQUEST", "binary market must have 0 or 2 outcome labels")
		}
	}

	if req.MarketType == domain.MarketTypeMulti {
		if len(req.OutcomeLabels) < 2 {
			return nil, apperrors.New("BAD_REQUEST", "multi market must have at least 2 outcome labels")
		}
	}

	// --- build domain objects ---
	now := time.Now()
	market := &domain.Market{
		ID:          generateID(),
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
		MarketType:  req.MarketType,
		Status:      domain.MarketStatusDraft,
		CreatorID:   req.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if !req.EndTime.IsZero() {
		t := req.EndTime
		market.ClosesAt = &t
	}

	// Build outcome list.
	var labels []string
	if req.MarketType == domain.MarketTypeBinary {
		if len(req.OutcomeLabels) == 2 {
			labels = req.OutcomeLabels
		} else {
			labels = []string{"Yes", "No"}
		}
	} else {
		labels = req.OutcomeLabels
	}

	outcomes := make([]*domain.Outcome, len(labels))
	for i, label := range labels {
		outcomes[i] = &domain.Outcome{
			ID:       generateID(),
			MarketID: market.ID,
			Label:    label,
			Index:    i,
			IsWinner: false,
		}
	}

	// --- persist inside a transaction ---
	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.marketRepo.Create(txCtx, market); err != nil {
			return err
		}
		if err := s.outcomeRepo.CreateBatch(txCtx, outcomes); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return market, nil
}

// ListMarkets returns markets matching the given filter along with a total count.
func (s *MarketService) ListMarkets(ctx context.Context, filter repository.MarketFilter) ([]*domain.Market, int64, error) {
	return s.marketRepo.List(ctx, filter)
}

// GetMarket retrieves a market and its outcomes by market ID.
// Returns a NOT_FOUND error if the market does not exist.
func (s *MarketService) GetMarket(ctx context.Context, id string) (*domain.Market, []*domain.Outcome, error) {
	market, err := s.marketRepo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	outcomes, err := s.outcomeRepo.ListByMarket(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	return market, outcomes, nil
}

// UpdateMarketStatus transitions a market to a new status after validating that
// the transition is allowed by the state machine. Returns BAD_REQUEST for
// invalid transitions and NOT_FOUND if the market does not exist.
func (s *MarketService) UpdateMarketStatus(ctx context.Context, id string, status domain.MarketStatus) error {
	market, err := s.marketRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if !isValidTransition(market.Status, status) {
		return apperrors.New("BAD_REQUEST",
			fmt.Sprintf("invalid status transition from %s to %s", market.Status, status))
	}

	market.Status = status
	market.UpdatedAt = time.Now()

	return s.marketRepo.Update(ctx, market)
}

// ResolveMarket atomically resolves a closed market by setting the winning
// outcome. The market must be in "closed" status, and the winning outcome ID
// must belong to this market. Returns BAD_REQUEST if preconditions are not met.
func (s *MarketService) ResolveMarket(ctx context.Context, marketID, winningOutcomeID string) error {
	market, err := s.marketRepo.GetByID(ctx, marketID)
	if err != nil {
		return err
	}

	if market.Status != domain.MarketStatusClosed {
		return apperrors.New("BAD_REQUEST",
			fmt.Sprintf("market must be closed to resolve, current status: %s", market.Status))
	}

	// Verify the winning outcome belongs to this market.
	outcomes, err := s.outcomeRepo.ListByMarket(ctx, marketID)
	if err != nil {
		return err
	}

	found := false
	for _, o := range outcomes {
		if o.ID == winningOutcomeID {
			found = true
			break
		}
	}
	if !found {
		return apperrors.New("BAD_REQUEST",
			fmt.Sprintf("outcome %s does not belong to market %s", winningOutcomeID, marketID))
	}

	// Persist resolution inside a transaction.
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		market.Status = domain.MarketStatusResolved
		market.ResolvedOutcomeID = &winningOutcomeID
		market.UpdatedAt = time.Now()

		if err := s.marketRepo.Update(txCtx, market); err != nil {
			return err
		}
		if err := s.outcomeRepo.SetWinner(txCtx, winningOutcomeID); err != nil {
			return err
		}
		return nil
	})
}
