package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/infra/storage"
	"github.com/tuusuario/candela-api/internal/repo"
)

var (
	ErrProtectedCannotBePublic = errors.New("Las cuentas protegidas no pueden ser públicas.")
	ErrCannotBlockYourself     = errors.New("No te puedes bloquear a ti mismo.")
	ErrProfilePrivate          = errors.New("Este perfil es privado.")
	ErrUserNotVisible          = errors.New("Usuario no encontrado.")
)

const jwtBlocklistKey = "jwt:blocklist:%s"

type UserService struct {
	userRepo       *repo.UserRepository
	friendshipRepo *repo.FriendshipRepository
	refreshRepo    *repo.RefreshTokenRepository
	storage        storage.ObjectStorage
	redis          *redis.Client
}

func NewUserService(
	ur *repo.UserRepository,
	fr *repo.FriendshipRepository,
	rr *repo.RefreshTokenRepository,
	st storage.ObjectStorage,
	rdb *redis.Client,
) *UserService {
	return &UserService{userRepo: ur, friendshipRepo: fr, refreshRepo: rr, storage: st, redis: rdb}
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

// Search finds users by username prefix for the friends search. Queries
// shorter than 2 characters return no results to avoid full-table scans and
// noisy matches. Privacy filtering happens in the repository.
func (s *UserService) Search(ctx context.Context, requesterID uuid.UUID, query string) ([]domain.UserSearchEntry, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return []domain.UserSearchEntry{}, nil
	}
	return s.userRepo.SearchByUsernamePrefix(ctx, requesterID, query, 10)
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

// DeleteAccount erases the user's content, anonymizes their identity, kills
// every session, and deletes their photos from object storage.
func (s *UserService) DeleteAccount(ctx context.Context, userID uuid.UUID, tokenExpiry time.Time) error {
	photoKeys, err := s.userRepo.DeleteAccount(ctx, userID)
	if err != nil {
		return err
	}

	// Sessions: the access token cannot be revoked, so blocklist it for the
	// rest of its (at most one hour) life; refresh tokens are revoked outright
	// so no new access token can be minted.
	ttl := time.Until(tokenExpiry)
	if ttl > 0 {
		key := fmt.Sprintf(jwtBlocklistKey, userID.String())
		s.redis.Set(ctx, key, "1", ttl)
	}
	if err := s.refreshRepo.RevokeAllForUser(ctx, userID); err != nil {
		log.Printf("user: could not revoke refresh tokens for %s: %v", userID, err)
	}

	// Photos are deleted after the transaction commits: the database is the
	// record of what was removed, and a failed object delete must not roll the
	// deletion back. Stragglers are reported for the storage sweeper.
	for _, key := range photoKeys {
		if err := s.storage.Delete(ctx, key); err != nil {
			log.Printf("user: could not delete photo %s for deleted account %s: %v", key, userID, err)
		}
	}

	// Invalidate ranking caches.
	s.redis.Del(ctx, globalRankingKey)
	s.redis.Del(ctx, fmt.Sprintf(friendRankingKey, userID.String()))
	return nil
}
