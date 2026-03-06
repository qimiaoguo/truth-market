package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	rankingv1 "github.com/truthmarket/truth-market/proto/gen/go/ranking/v1"
)

// RankingHandler handles HTTP requests for ranking/leaderboard endpoints and
// delegates to the ranking-svc via gRPC.
type RankingHandler struct {
	rankingClient rankingv1.RankingServiceClient
}

// NewRankingHandler creates a new RankingHandler with the given gRPC ranking client.
func NewRankingHandler(rankingClient rankingv1.RankingServiceClient) *RankingHandler {
	return &RankingHandler{rankingClient: rankingClient}
}

// rankingResponse is the JSON representation of a user ranking entry.
type rankingResponse struct {
	UserID        string `json:"user_id"`
	WalletAddress string `json:"wallet_address"`
	UserType      string `json:"user_type"`
	Rank          int64  `json:"rank"`
	Value         string `json:"value"`
}

// dimensionRankResponse is the JSON representation of a single dimension rank.
type dimensionRankResponse struct {
	Dimension string `json:"dimension"`
	Rank      int64  `json:"rank"`
	Value     string `json:"value"`
}

// GetLeaderboard handles GET /api/v1/rankings.
func (h *RankingHandler) GetLeaderboard(c *gin.Context) {
	req := &rankingv1.GetLeaderboardRequest{}

	if d := c.Query("dimension"); d != "" {
		if v, ok := rankingv1.RankDimension_value[d]; ok {
			req.Dimension = rankingv1.RankDimension(v)
		}
	}
	if ut := c.Query("user_type"); ut != "" {
		if v, ok := rankingv1.UserTypeFilter_value[ut]; ok {
			req.UserType = rankingv1.UserTypeFilter(v)
		}
	}
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			req.Page = int32(v)
		}
	}
	if pp := c.Query("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil {
			req.PerPage = int32(v)
		}
	}

	resp, err := h.rankingClient.GetLeaderboard(c.Request.Context(), req)
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	rankings := make([]rankingResponse, 0, len(resp.GetRankings()))
	for _, r := range resp.GetRankings() {
		rankings = append(rankings, rankingResponse{
			UserID:        r.GetUserId(),
			WalletAddress: r.GetWalletAddress(),
			UserType:      r.GetUserType(),
			Rank:          r.GetRank(),
			Value:         r.GetValue(),
		})
	}

	result := gin.H{
		"rankings": rankings,
		"total":    resp.GetTotal(),
	}
	data, _ := json.Marshal(result)

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}

// GetUserRanking handles GET /api/v1/rankings/user/:id.
func (h *RankingHandler) GetUserRanking(c *gin.Context) {
	userID := c.Param("id")

	resp, err := h.rankingClient.GetUserRanking(c.Request.Context(), &rankingv1.GetUserRankingRequest{
		UserId: userID,
	})
	if err != nil {
		handleGRPCError(c, err)
		return
	}

	ranks := make([]dimensionRankResponse, 0, len(resp.GetRanks()))
	for _, r := range resp.GetRanks() {
		ranks = append(ranks, dimensionRankResponse{
			Dimension: r.GetDimension().String(),
			Rank:      r.GetRank(),
			Value:     r.GetValue(),
		})
	}

	result := gin.H{
		"ranks": ranks,
	}
	data, _ := json.Marshal(result)

	c.JSON(http.StatusOK, gin.H{
		"ok":   true,
		"data": json.RawMessage(data),
	})
}
