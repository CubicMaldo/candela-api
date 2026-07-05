package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
)

const (
	globalRankingTTL   = time.Hour
	friendRankingTTL   = 5 * time.Minute
	globalRankingKey   = "rankings:global"
	friendRankingKey   = "rankings:friends:%s"
	globalDefaultLimit = 50
)

var rankingOptInDisabledReason = "ranking_opt_in_disabled"

type RankingService struct {
	rankingRepo *repo.RankingRepository
	userRepo    *repo.UserRepository
	redis       *redis.Client
}

func NewRankingService(rr *repo.RankingRepository, ur *repo.UserRepository, rdb *redis.Client) *RankingService {
	return &RankingService{rankingRepo: rr, userRepo: ur, redis: rdb}
}

// InvalidateRankingCache clears both the global ranking cache and the user's
// friend ranking cache. Called when a user changes ranking_opt_in.
func (s *RankingService) InvalidateRankingCache(ctx context.Context, userID uuid.UUID) error {
	if err := s.redis.Del(ctx, globalRankingKey).Err(); err != nil {
		return err
	}
	cacheKey := fmt.Sprintf(friendRankingKey, userID.String())
	return s.redis.Del(ctx, cacheKey).Err()
}

// UpdateOptIn changes the user's ranking_opt_in flag and invalidates the
// global and friend ranking caches so the change takes effect immediately.
func (s *RankingService) UpdateOptIn(ctx context.Context, userID uuid.UUID, optIn bool) error {
	if err := s.userRepo.UpdateRankingOptIn(ctx, userID, optIn); err != nil {
		return err
	}
	return s.InvalidateRankingCache(ctx, userID)
}

func (s *RankingService) GetGlobal(ctx context.Context, userID uuid.UUID) (*domain.RankingsResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ranking: failed to fetch user: %w", err)
	}
	if !user.RankingOptIn {
		entries, total, err := s.fetchGlobal(ctx)
		if err != nil {
			return nil, err
		}
		reason := rankingOptInDisabledReason
		return &domain.RankingsResponse{
			Rankings:       entries,
			Total:          total,
			UserRank:       nil,
			UserRankReason: &reason,
		}, nil
	}

	entries, total, err := s.fetchGlobal(ctx)
	if err != nil {
		return nil, err
	}

	userRank, _ := s.rankingRepo.GetUserGlobalRank(ctx, userID)

	return &domain.RankingsResponse{
		Rankings: entries,
		Total:    total,
		UserRank: userRank,
	}, nil
}

// fetchGlobal returns the global rankings, using the Redis cache when available.
func (s *RankingService) fetchGlobal(ctx context.Context) ([]domain.RankingEntry, int, error) {
	cached, err := s.redis.Get(ctx, globalRankingKey).Result()
	if err == nil {
		var rankings []domain.RankingEntry
		if err := json.Unmarshal([]byte(cached), &rankings); err == nil {
			return rankings, len(rankings), nil
		}
	}

	entries, total, err := s.rankingRepo.GetGlobal(ctx, globalDefaultLimit, 0)
	if err != nil {
		return nil, 0, fmt.Errorf("ranking: failed to fetch global: %w", err)
	}

	if data, err := json.Marshal(entries); err == nil {
		s.redis.Set(ctx, globalRankingKey, data, globalRankingTTL)
	}

	return entries, total, nil
}

func (s *RankingService) GetFriends(ctx context.Context, userID uuid.UUID) (*domain.RankingsResponse, error) {
	cacheKey := fmt.Sprintf(friendRankingKey, userID.String())

	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var response domain.RankingsResponse
		if err := json.Unmarshal([]byte(cached), &response); err == nil {
			return &response, nil
		}
	}

	entries, err := s.rankingRepo.GetFriends(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("ranking: failed to fetch friends: %w", err)
	}

	userRank, _ := s.rankingRepo.GetUserFriendRank(ctx, userID)

	response := &domain.RankingsResponse{
		Rankings: entries,
		Total:    len(entries),
		UserRank: userRank,
	}

	if data, err := json.Marshal(response); err == nil {
		s.redis.Set(ctx, cacheKey, data, friendRankingTTL)
	}

	return response, nil
}
