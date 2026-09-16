package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"

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

type RankingRepo interface {
	GetGlobal(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error)
	GetUserGlobalRank(ctx context.Context, userID uuid.UUID) (*domain.RankingEntry, error)
	GetFriends(ctx context.Context, userID uuid.UUID) ([]domain.RankingEntry, error)
	GetUserFriendRank(ctx context.Context, userID uuid.UUID) (*domain.RankingEntry, error)
}

type RankingUserRepo interface {
	GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
	UpdateRankingOptIn(ctx context.Context, userID uuid.UUID, optIn bool) error
}

type RankingService struct {
	rankingRepo RankingRepo
	userRepo    RankingUserRepo
	redis       *redis.Client
	sf          singleflight.Group
}

func NewRankingService(rr *repo.RankingRepository, ur *repo.UserRepository, rdb *redis.Client) *RankingService {
	return NewRankingServiceWithCustom(rr, ur, rdb)
}

func NewRankingServiceWithCustom(rr RankingRepo, ur RankingUserRepo, rdb *redis.Client) *RankingService {
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

type globalResult struct {
	entries []domain.RankingEntry
	total   int
}

// fetchGlobal returns the global rankings, using the Redis cache when available,
// and protects against cache stampedes / thundering herd using singleflight.
func (s *RankingService) fetchGlobal(ctx context.Context) ([]domain.RankingEntry, int, error) {
	// 1. Fast path: check Redis cache
	cached, err := s.redis.Get(ctx, globalRankingKey).Result()
	if err == nil {
		var rankings []domain.RankingEntry
		if err := json.Unmarshal([]byte(cached), &rankings); err == nil {
			return rankings, len(rankings), nil
		}
	}

	// 2. Coalesce concurrent cache misses into a single database query
	val, err, _ := s.sf.Do(globalRankingKey, func() (interface{}, error) {
		// Re-check cache inside singleflight: a preceding flight may have just populated it
		if cached, cErr := s.redis.Get(ctx, globalRankingKey).Result(); cErr == nil {
			var rankings []domain.RankingEntry
			if uErr := json.Unmarshal([]byte(cached), &rankings); uErr == nil {
				return globalResult{entries: rankings, total: len(rankings)}, nil
			}
		}

		entries, total, qErr := s.rankingRepo.GetGlobal(ctx, globalDefaultLimit, 0)
		if qErr != nil {
			return nil, fmt.Errorf("ranking: failed to fetch global: %w", qErr)
		}

		if data, mErr := json.Marshal(entries); mErr == nil {
			s.redis.Set(ctx, globalRankingKey, data, globalRankingTTL)
		}

		return globalResult{entries: entries, total: total}, nil
	})

	if err != nil {
		return nil, 0, err
	}

	res := val.(globalResult)
	return res.entries, res.total, nil
}

func (s *RankingService) GetFriends(ctx context.Context, userID uuid.UUID) (*domain.RankingsResponse, error) {
	cacheKey := fmt.Sprintf(friendRankingKey, userID.String())

	// 1. Fast path: check Redis cache
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var response domain.RankingsResponse
		if err := json.Unmarshal([]byte(cached), &response); err == nil {
			return &response, nil
		}
	}

	// 2. Coalesce concurrent cache misses for this friend list into a single query
	val, err, _ := s.sf.Do(cacheKey, func() (interface{}, error) {
		if cached, cErr := s.redis.Get(ctx, cacheKey).Result(); cErr == nil {
			var response domain.RankingsResponse
			if uErr := json.Unmarshal([]byte(cached), &response); uErr == nil {
				return &response, nil
			}
		}

		entries, qErr := s.rankingRepo.GetFriends(ctx, userID)
		if qErr != nil {
			return nil, fmt.Errorf("ranking: failed to fetch friends: %w", qErr)
		}

		userRank, _ := s.rankingRepo.GetUserFriendRank(ctx, userID)

		response := &domain.RankingsResponse{
			Rankings: entries,
			Total:    len(entries),
			UserRank: userRank,
		}

		if data, mErr := json.Marshal(response); mErr == nil {
			s.redis.Set(ctx, cacheKey, data, friendRankingTTL)
		}

		return response, nil
	})

	if err != nil {
		return nil, err
	}

	return val.(*domain.RankingsResponse), nil
}
