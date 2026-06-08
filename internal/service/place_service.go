package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

type PlaceService struct {
	placeRepo     *repo.PlaceRepository
	followService *FollowService
	checkinRepo   *repo.CheckInRepository
}

func NewPlaceService(pr *repo.PlaceRepository, fs *FollowService, cr *repo.CheckInRepository) *PlaceService {
	return &PlaceService{placeRepo: pr, followService: fs, checkinRepo: cr}
}

func (s *PlaceService) SearchNearby(ctx context.Context, req domain.SearchPlacesRequest) ([]domain.Place, error) {
	// PostGIS evalúa las distancias geográficas en metros
	radiusMeters := req.RadiusKM * 1000.0

	// Delegamos la validación espacial al motor robusto de PostgreSQL
	return s.placeRepo.FindNearby(ctx, req.Lat, req.Lon, radiusMeters, req.Category)
}

// GetDetail returns the full detail of a single place, including whether the
// requester follows it and how many check-ins it has had in the last 4 hours.
func (s *PlaceService) GetDetail(ctx context.Context, requesterID, placeID uuid.UUID) (*domain.PlaceDetail, error) {
	place, err := s.placeRepo.GetByID(ctx, placeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlaceNotFound
		}
		return nil, err
	}
	if !place.IsActive {
		return nil, ErrPlaceNotFound
	}

	isFollowed, err := s.followService.IsFollowing(ctx, requesterID, placeID)
	if err != nil {
		return nil, err
	}
	followerCount, err := s.followService.GetFollowerCount(ctx, placeID)
	if err != nil {
		return nil, err
	}
	activeCount, err := s.checkinRepo.GetActiveCheckinsCount(ctx, placeID)
	if err != nil {
		return nil, err
	}
	lastCheckinAt, err := s.checkinRepo.GetLastCheckinAt(ctx, placeID)
	if err != nil {
		return nil, err
	}

	return &domain.PlaceDetail{
		PlaceID:             place.PlaceID,
		Name:                place.Name,
		Category:            place.Category,
		Lat:                 place.Lat,
		Lon:                 place.Lon,
		GeofenceMeters:      place.GeofenceMeters,
		IsVerified:          place.IsVerified,
		IsActive:            place.IsActive,
		IsFollowed:          isFollowed,
		FollowerCount:       followerCount,
		ActiveCheckinsCount: activeCount,
		LastCheckinAt:       lastCheckinAt,
	}, nil
}

// GetEventFeed returns the live event feed for a place: check-ins with photos
// from the last 4 hours, newest first.
func (s *PlaceService) GetEventFeed(ctx context.Context, placeID uuid.UUID, limit int, before *time.Time) (*domain.PlaceEventFeed, error) {
	place, err := s.placeRepo.GetByID(ctx, placeID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPlaceNotFound
		}
		return nil, err
	}

	events, err := s.checkinRepo.GetEventFeed(ctx, placeID, limit, before)
	if err != nil {
		return nil, err
	}
	if events == nil {
		events = []domain.CheckInEvent{}
	}

	activeCount, err := s.checkinRepo.GetActiveCheckinsCount(ctx, placeID)
	if err != nil {
		return nil, err
	}

	var cursor *time.Time
	if len(events) >= limit && len(events) > 0 {
		t := events[len(events)-1].Timestamp
		cursor = &t
	}

	return &domain.PlaceEventFeed{
		PlaceID:    place.PlaceID,
		PlaceName:  place.Name,
		IsActive:   activeCount > 0,
		Events:     events,
		Total:      len(events),
		NextCursor: cursor,
	}, nil
}
