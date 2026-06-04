package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

type ProfileService struct {
	userRepo      *repo.UserRepository
	feedRepo      *repo.FeedRepository
	checkinRepo   *repo.CheckInRepository
	challengeRepo *repo.ChallengeRepository
}

func NewProfileService(
	ur *repo.UserRepository,
	fr *repo.FeedRepository,
	cr *repo.CheckInRepository,
	ch *repo.ChallengeRepository,
) *ProfileService {
	return &ProfileService{userRepo: ur, feedRepo: fr, checkinRepo: cr, challengeRepo: ch}
}

func (s *ProfileService) GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.MyProfile, error) {
	return s.userRepo.GetMyProfile(ctx, userID)
}

// GetFeed merges check-in and challenge events from accepted friends, sorted newest-first.
// Returns the event slice and the cursor for the next page (nil if no more results).
func (s *ProfileService) GetFeed(ctx context.Context, userID uuid.UUID, limit int, before *time.Time) ([]interface{}, *time.Time, error) {
	checkins, err := s.feedRepo.GetFeedCheckIns(ctx, userID, limit, before)
	if err != nil {
		return nil, nil, err
	}

	challenges, err := s.feedRepo.GetFeedChallenges(ctx, userID, limit, before)
	if err != nil {
		return nil, nil, err
	}

	// Merge into a common sortable slice.
	type entry struct {
		ts   time.Time
		data interface{}
	}

	merged := make([]entry, 0, len(checkins)+len(challenges))
	for _, c := range checkins {
		merged = append(merged, entry{c.Timestamp, c})
	}
	for _, c := range challenges {
		merged = append(merged, entry{c.Timestamp, c})
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].ts.After(merged[j].ts)
	})

	if len(merged) > limit {
		merged = merged[:limit]
	}

	events := make([]interface{}, len(merged))
	for i, e := range merged {
		events[i] = e.data
	}

	var cursor *time.Time
	if len(merged) >= limit && len(merged) > 0 {
		t := merged[len(merged)-1].ts
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

func (s *ProfileService) GetMyChallenges(ctx context.Context, userID uuid.UUID, statusFilter string) ([]domain.MyChallenge, error) {
	// Lazy expiration: mark past-deadline challenges as FAILED before returning.
	if err := s.challengeRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}

	results, err := s.challengeRepo.GetMyChallenges(ctx, userID, statusFilter)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.MyChallenge{}
	}
	return results, nil
}
