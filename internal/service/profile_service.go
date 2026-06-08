package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

type ProfileService struct {
	userRepo    *repo.UserRepository
	feedRepo    *repo.FeedRepository
	checkinRepo *repo.CheckInRepository
	planRepo    *repo.PlanRepository
}

func NewProfileService(
	ur *repo.UserRepository,
	fr *repo.FeedRepository,
	cr *repo.CheckInRepository,
	pr *repo.PlanRepository,
) *ProfileService {
	return &ProfileService{userRepo: ur, feedRepo: fr, checkinRepo: cr, planRepo: pr}
}

func (s *ProfileService) GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.MyProfile, error) {
	return s.userRepo.GetMyProfile(ctx, userID)
}

// GetFeed returns the three-layer check-in feed (friends, followed places, city
// activity), newest-first within each layer. Returns the event slice and the
// cursor for the next page (nil if no more results).
func (s *ProfileService) GetFeed(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]domain.CheckInEvent, *time.Time, error) {
	events, err := s.feedRepo.GetFeedCheckIns(ctx, userID, limit, before)
	if err != nil {
		return nil, nil, err
	}
	if events == nil {
		events = []domain.CheckInEvent{}
	}

	var cursor *time.Time
	if len(events) >= limit && len(events) > 0 {
		t := events[len(events)-1].Timestamp
		cursor = &t
	}

	return events, cursor, nil
}

func (s *ProfileService) GetMyCheckIns(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]domain.MyCheckIn, *time.Time, error) {
	results, err := s.checkinRepo.GetMyCheckIns(ctx, userID, limit, before)
	if err != nil {
		return nil, nil, err
	}
	if results == nil {
		results = []domain.MyCheckIn{}
	}

	var cursor *time.Time
	if len(results) >= limit && len(results) > 0 {
		t := results[len(results)-1].CheckedInAt
		cursor = &t
	}

	return results, cursor, nil
}

func (s *ProfileService) GetMyPlans(ctx context.Context, userID uuid.UUID, statusFilter string) ([]domain.MyPlan, error) {
	// Lazy expiration: mark past-deadline plans as FAILED before returning.
	if err := s.planRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}

	results, err := s.planRepo.GetMyPlans(ctx, userID, statusFilter)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.MyPlan{}
	}
	return results, nil
}
