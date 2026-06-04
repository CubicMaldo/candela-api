package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

const (
	globalRankingTTL   = time.Hour
	friendRankingTTL   = 5 * time.Minute
	globalRankingKey   = "rankings:global"
	friendRankingKey   = "rankings:friends:%s"
	globalDefaultLimit = 50
)

type RankingService struct {
	rankingRepo *repo.RankingRepository
	redis       *redis.Client
}

func NewRankingService(rr *repo.RankingRepository, rdb *redis.Client) *RankingService {
	return &RankingService{rankingRepo: rr, redis: rdb}
}

func (s *RankingService) GetGlobal(ctx context.Context, userID uuid.UUID) (*domain.RankingsResponse, error) {
	cached, err := s.redis.Get(ctx, globalRankingKey).Result()
	if err == nil {
		var rankings []domain.RankingEntry
		if err := json.Unmarshal([]byte(cached), &rankings); err == nil {
			userRank, _ := s.rankingRepo.GetUserGlobalRank(ctx, userID)
			return &domain.RankingsResponse{
				Rankings: rankings,
				Total:    len(rankings),
				UserRank: userRank,
			}, nil
		}
	}

	entries, total, err := s.rankingRepo.GetGlobal(ctx, globalDefaultLimit, 0)
	if err != nil {
		return nil, fmt.Errorf("ranking: failed to fetch global: %w", err)
	}

	if data, err := json.Marshal(entries); err == nil {
		s.redis.Set(ctx, globalRankingKey, data, globalRankingTTL)
	}

	userRank, _ := s.rankingRepo.GetUserGlobalRank(ctx, userID)

	return &domain.RankingsResponse{
		Rankings: entries,
		Total:    total,
		UserRank: userRank,
	}, nil
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
