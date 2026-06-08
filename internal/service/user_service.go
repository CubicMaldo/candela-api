package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

var (
	ErrProtectedCannotBePublic = errors.New("protected accounts cannot set privacy to PUBLIC")
	ErrCannotBlockYourself     = errors.New("you cannot block yourself")
	ErrProfilePrivate          = errors.New("this profile is private")
	ErrUserNotVisible          = errors.New("Usuario no encontrado.")
)

const jwtBlocklistKey = "jwt:blocklist:%s"

type UserService struct {
	userRepo       *repo.UserRepository
	friendshipRepo *repo.FriendshipRepository
	redis          *redis.Client
}

func NewUserService(ur *repo.UserRepository, fr *repo.FriendshipRepository, rdb *redis.Client) *UserService {
	return &UserService{userRepo: ur, friendshipRepo: fr, redis: rdb}
}

func (s *UserService) UpdatePrivacy(ctx context.Context, userID uuid.UUID, isProtected bool, mode string) error {
	if isProtected && mode == "PUBLIC" {
		return ErrProtectedCannotBePublic
	}
	if err := s.userRepo.UpdatePrivacyMode(ctx, userID, mode); err != nil {
		return err
	}
	// Invalidate both caches — global because PUBLIC eligibility may change.
	s.redis.Del(ctx, globalRankingKey)
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, userID.String()))
	return nil
}

func (s *UserService) GetProfile(ctx context.Context, requesterID uuid.UUID, username string) (*domain.ProfileResult, error) {
	target, checkInCount, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, ErrUserNotVisible
	}

	blocked, err := s.friendshipRepo.IsBlocked(ctx, requesterID, target.UserID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrUserNotVisible
	}

	isSelf := requesterID == target.UserID
	isFriend := false
	if !isSelf {
		isFriend, err = s.friendshipRepo.AreFriends(ctx, requesterID, target.UserID)
		if err != nil {
			return nil, err
		}
	}

	full := &domain.FullUserProfile{
		UserID:             target.UserID,
		Username:           target.Username,
		BadgeLevel:         target.BadgeLevel,
		StreakWeeksCurrent: target.StreakWeeksCurrent,
		StreakWeeksMax:     target.StreakWeeksMax,
		CheckInCount:       checkInCount,
		CreatedAt:          target.CreatedAt,
	}
	// points_total is only included when the target opted into the ranking —
	// otherwise it must be omitted from the JSON entirely, not returned as null.
	if target.RankingOptIn {
		points := target.PointsTotal
		full.PointsTotal = &points
	}
	partial := &domain.PartialUserProfile{
		Username:   target.Username,
		BadgeLevel: target.BadgeLevel,
	}

	switch target.PrivacyMode {
	case "PUBLIC":
		return &domain.ProfileResult{Full: full}, nil
	case "FRIENDS":
		if isSelf || isFriend {
			return &domain.ProfileResult{Full: full}, nil
		}
		return &domain.ProfileResult{Partial: partial}, nil
	case "PRIVATE":
		if isSelf {
			return &domain.ProfileResult{Full: full}, nil
		}
		if isFriend {
			return &domain.ProfileResult{Partial: partial}, nil
		}
		return nil, ErrProfilePrivate
	}
	return nil, ErrUserNotVisible
}

func (s *UserService) Block(ctx context.Context, requesterID, targetID uuid.UUID) error {
	if requesterID == targetID {
		return ErrCannotBlockYourself
	}
	if err := s.friendshipRepo.Block(ctx, requesterID, targetID); err != nil {
		return err
	}
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, requesterID.String()))
	return nil
}

func (s *UserService) Unblock(ctx context.Context, requesterID, targetID uuid.UUID) error {
	if err := s.friendshipRepo.Unblock(ctx, requesterID, targetID); err != nil {
		return err
	}
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, requesterID.String()))
	return nil
}

func (s *UserService) DeleteAccount(ctx context.Context, userID uuid.UUID, tokenExpiry time.Time) error {
	if err := s.userRepo.SoftDelete(ctx, userID); err != nil {
		return err
	}

	// Blocklist the user's active token for its remaining lifetime.
	ttl := time.Until(tokenExpiry)
	if ttl > 0 {
		key := fmt.Sprintf(jwtBlocklistKey, userID.String())
		s.redis.Set(ctx, key, "1", ttl)
	}

	// Invalidate ranking caches.
	s.redis.Del(ctx, globalRankingKey)
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, userID.String()))
	return nil
}
