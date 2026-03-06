package redis

import (
	"context"
	"encoding/json"
	"fmt"

	goredis "github.com/redis/go-redis/v9"
	"github.com/truthmarket/truth-market/pkg/domain"
)

const rankingKeyPrefix = "ranking:"

// RankingCache provides a Redis sorted-set-backed cache for user rankings,
// enabling fast leaderboard queries by dimension and user type.
type RankingCache struct {
	client *goredis.Client
}

// NewRankingCache returns a new RankingCache backed by the given Redis client.
func NewRankingCache(client *goredis.Client) *RankingCache {
	return &RankingCache{client: client}
}

func rankingKey(dimension, userType string) string {
	return fmt.Sprintf("%s%s:%s", rankingKeyPrefix, dimension, userType)
}

// rankingMember is the JSON payload stored alongside each sorted-set member.
type rankingMember struct {
	UserID   string `json:"user_id"`
	UserType string `json:"user_type"`
}

// SetRankings replaces the cached rankings for the given dimension and user
// type. The existing sorted set is deleted first to ensure consistency.
func (c *RankingCache) SetRankings(ctx context.Context, dimension string, userType string, rankings []domain.UserRanking) error {
	key := rankingKey(dimension, userType)

	pipe := c.client.Pipeline()
	pipe.Del(ctx, key)

	for _, r := range rankings {
		member, err := json.Marshal(rankingMember{
			UserID:   r.UserID,
			UserType: string(r.UserType),
		})
		if err != nil {
			return fmt.Errorf("marshal ranking member: %w", err)
		}
		score, _ := r.Value.Float64()
		pipe.ZAdd(ctx, key, goredis.Z{
			Score:  score,
			Member: string(member),
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetTopN returns the top N users from the cached ranking sorted set,
// ordered from highest to lowest score.
func (c *RankingCache) GetTopN(ctx context.Context, dimension, userType string, n int) ([]domain.UserRanking, error) {
	key := rankingKey(dimension, userType)

	results, err := c.client.ZRevRangeWithScores(ctx, key, 0, int64(n-1)).Result()
	if err != nil {
		return nil, err
	}

	rankings := make([]domain.UserRanking, 0, len(results))
	for i, z := range results {
		var m rankingMember
		if err := json.Unmarshal([]byte(z.Member.(string)), &m); err != nil {
			return nil, fmt.Errorf("unmarshal ranking member: %w", err)
		}
		rankings = append(rankings, domain.UserRanking{
			UserID:    m.UserID,
			UserType:  domain.UserType(m.UserType),
			Dimension: domain.RankDimension(dimension),
			Rank:      i + 1,
		})
	}

	return rankings, nil
}

// GetUserRank returns the 1-based rank of a specific user within the given
// dimension and user-type sorted set. Returns 0 if the user is not found.
func (c *RankingCache) GetUserRank(ctx context.Context, dimension, userType, userID string) (int64, error) {
	key := rankingKey(dimension, userType)

	member, err := json.Marshal(rankingMember{
		UserID:   userID,
		UserType: userType,
	})
	if err != nil {
		return 0, fmt.Errorf("marshal ranking member: %w", err)
	}

	rank, err := c.client.ZRevRank(ctx, key, string(member)).Result()
	if err == goredis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return rank + 1, nil
}
