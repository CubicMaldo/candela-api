package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
)

var (
	ErrPlaceNotFound = errors.New("Este lugar no está disponible en este momento.")
	ErrNotFollowing  = errors.New("No seguías este lugar.")
)

type FollowService struct {
	followRepo *repo.FollowRepository
	placeRepo  *repo.PlaceRepository
}

func NewFollowService(fr *repo.FollowRepository, pr *repo.PlaceRepository) *FollowService {
	return &FollowService{followRepo: fr, placeRepo: pr}
}

// Follow makes followerID follow placeID. Validates the place exists first.
// Idempotent — following an already-followed place succeeds without error.
// Returns the place's resulting follower count.
func (s *FollowService) Follow(ctx context.Context, followerID, placeID uuid.UUID) (int, error) {
	active, err := s.placeRepo.IsActive(ctx, placeID)
	if err != nil {
		return 0, err
	}
	if !active {
		return 0, ErrPlaceNotFound
	}
	if err := s.followRepo.Follow(ctx, followerID, placeID); err != nil {
		return 0, err
	}
	return s.followRepo.GetFollowerCount(ctx, placeID)
}

// Unfollow removes the follow relationship between followerID and placeID.
// Returns ErrNotFollowing if no such relationship exists.
// Returns the place's resulting follower count.
func (s *FollowService) Unfollow(ctx context.Context, followerID, placeID uuid.UUID) (int, error) {
	following, err := s.followRepo.IsFollowing(ctx, followerID, placeID)
	if err != nil {
		return 0, err
	}
	if !following {
		return 0, ErrNotFollowing
	}
	if err := s.followRepo.Unfollow(ctx, followerID, placeID); err != nil {
		return 0, err
	}
	return s.followRepo.GetFollowerCount(ctx, placeID)
}

// IsFollowing reports whether followerID currently follows placeID.
func (s *FollowService) IsFollowing(ctx context.Context, followerID, placeID uuid.UUID) (bool, error) {
	return s.followRepo.IsFollowing(ctx, followerID, placeID)
}

// GetFollowerCount returns the number of users following placeID.
func (s *FollowService) GetFollowerCount(ctx context.Context, placeID uuid.UUID) (int, error) {
	return s.followRepo.GetFollowerCount(ctx, placeID)
}

// GetFollowedPlaces returns the full place details for every place followerID follows.
func (s *FollowService) GetFollowedPlaces(ctx context.Context, followerID uuid.UUID) ([]domain.Place, error) {
	placeIDs, err := s.followRepo.GetFollowedPlaceIDs(ctx, followerID)
	if err != nil {
		return nil, err
	}

	places := make([]domain.Place, 0, len(placeIDs))
	for _, id := range placeIDs {
		place, err := s.placeRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		places = append(places, *place)
	}
	return places, nil
}
