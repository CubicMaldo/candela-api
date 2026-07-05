package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/infra/storage"
	"github.com/tuusuario/candela-api/internal/repo"
)

var (
	ErrNotWithinGeofence   = errors.New("No te ubico bien ahí. Acércate un poco más.")
	ErrAlreadyCheckedIn    = errors.New("Ya estuviste aquí hoy.")
	ErrInvalidCheckInNonce = errors.New("El tiempo se acabó. Inténtalo de nuevo.")
	ErrPhotoNotLive        = errors.New("La foto debe tomarse en el lugar en este momento.")
	ErrCheckInNotFound     = errors.New("Check-in no encontrado.")
	ErrNotCheckInOwner     = errors.New("No puedes modificar el check-in de otro usuario.")
	ErrEventClosed         = errors.New("El evento ya cerró, no puedes cambiar esta foto.")
)

const (
	pointsBase        = 10
	pointsFirstOfDay  = 5
	pointsWithPhoto   = 5
	nonceTTL          = 90 * time.Second
	photoViewDuration = 24 * time.Hour
	uploadURLDuration = 90 * time.Second
)

type CheckInService struct {
	checkinRepo *repo.CheckInRepository
	planRepo    *repo.PlanRepository
	userRepo    *repo.UserRepository
	storage     storage.ObjectStorage
	redis       *redis.Client
}

func NewCheckInService(cr *repo.CheckInRepository, pr *repo.PlanRepository, ur *repo.UserRepository, st storage.ObjectStorage, rdb *redis.Client) *CheckInService {
	return &CheckInService{checkinRepo: cr, planRepo: pr, userRepo: ur, storage: st, redis: rdb}
}

// startOfWeek returns midnight UTC of the Monday of the week containing t.
func startOfWeek(t time.Time) time.Time {
	t = t.UTC()
	weekday := int(t.Weekday())
	// time.Sunday == 0; treat Monday as the first day of the week.
	daysSinceMonday := (weekday + 6) % 7
	monday := t.AddDate(0, 0, -daysSinceMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// updateWeeklyStreak detects week boundaries (Monday-based), updates the user's
// weekly streak counters, and returns the points multiplier and weekly consistency bonus.
func (s *CheckInService) updateWeeklyStreak(ctx context.Context, userID uuid.UUID, now time.Time) (multiplier float64, weeklyBonus int, err error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return 0, 0, err
	}

	weekStart := startOfWeek(now)

	if user.StreakWeekStart == nil || user.StreakWeekStart.Before(weekStart) {
		lastWeekStart := weekStart.AddDate(0, 0, -7)
		hadActivityLastWeek := user.StreakLastCheckin != nil &&
			!user.StreakLastCheckin.Before(lastWeekStart)

		if hadActivityLastWeek {
			user.StreakWeeksCurrent++
		} else {
			user.StreakWeeksCurrent = 1
		}
		if user.StreakWeeksCurrent > user.StreakWeeksMax {
			user.StreakWeeksMax = user.StreakWeeksCurrent
		}
		user.StreakWeekCheckins = 0
		user.StreakWeekStart = &weekStart
	}

	user.StreakWeekCheckins++
	user.StreakLastCheckin = &now

	switch {
	case user.StreakWeekCheckins >= 4:
		multiplier = 2.0
	case user.StreakWeekCheckins >= 2:
		multiplier = 1.5
	default:
		multiplier = 1.0
	}

	if user.StreakWeekCheckins == 1 {
		switch {
		case user.StreakWeeksCurrent >= 8:
			weeklyBonus = 10
		case user.StreakWeeksCurrent >= 4:
			weeklyBonus = 5
		default:
			weeklyBonus = 0
		}
	}

	err = s.checkinRepo.UpdateStreakFields(ctx, user)
	return multiplier, weeklyBonus, err
}

// Initiate validates GPS and returns a presigned upload URL.
// The nonce is stored in Redis for 90 seconds to bind the upload to this session.
func (s *CheckInService) Initiate(
	ctx context.Context,
	userID, placeID uuid.UUID,
	isProtected bool,
	req domain.CheckInInitiateRequest,
) (*domain.CheckInInitiateResponse, error) {

	if isProtected {
		restricted, err := s.checkinRepo.IsRestrictedForMinors(ctx, placeID)
		if err != nil {
			return nil, err
		}
		if restricted {
			return nil, errors.New("Este lugar no está disponible en este momento.")
		}
	}

	within, err := s.checkinRepo.IsWithinGeofence(ctx, placeID, req.Lat, req.Lon)
	if err != nil {
		return nil, err
	}
	if !within {
		return nil, ErrNotWithinGeofence
	}

	alreadyIn, err := s.checkinRepo.HasCheckedInToday(ctx, userID, placeID)
	if err != nil {
		return nil, err
	}
	if alreadyIn {
		return nil, ErrAlreadyCheckedIn
	}

	nonce := uuid.New().String()
	photoKey := fmt.Sprintf("checkins/%s/%s/%s.jpg",
		userID.String(),
		time.Now().UTC().Format("2006-01-02"),
		uuid.New().String(),
	)

	nonceValue := fmt.Sprintf("%s:%s:%s", userID, placeID, photoKey)
	if err := s.redis.Set(ctx, "checkin:nonce:"+nonce, nonceValue, nonceTTL).Err(); err != nil {
		return nil, fmt.Errorf("failed to store check-in session: %w", err)
	}

	uploadURL, err := s.storage.PresignedPutURL(ctx, photoKey, uploadURLDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate upload URL: %w", err)
	}

	return &domain.CheckInInitiateResponse{
		CheckInNonce: nonce,
		UploadURL:    uploadURL,
		PhotoKey:     photoKey,
	}, nil
}

// Confirm validates the nonce, saves the check-in record, and awards points.
func (s *CheckInService) Confirm(
	ctx context.Context,
	userID, placeID uuid.UUID,
	req domain.CheckInConfirmRequest,
) (*domain.CheckInResult, error) {

	nonceKey := "checkin:nonce:" + req.CheckInNonce
	val, err := s.redis.Get(ctx, nonceKey).Result()
	if err == redis.Nil {
		return nil, ErrInvalidCheckInNonce
	}
	if err != nil {
		return nil, fmt.Errorf("failed to validate check-in session: %w", err)
	}

	expected := fmt.Sprintf("%s:%s:%s", userID, placeID, req.PhotoKey)
	if val != expected {
		return nil, ErrInvalidCheckInNonce
	}

	// Single-use: delete immediately after validation
	s.redis.Del(ctx, nonceKey)

	now := time.Now().UTC()
	diff := now.Sub(req.ExifTimestamp)
	if diff < 0 {
		diff = -diff
	}
	if diff > 60*time.Second {
		return nil, ErrPhotoNotLive
	}

	isFirstOfDay, err := s.checkinRepo.IsFirstCheckInToday(ctx, userID)
	if err != nil {
		return nil, err
	}

	basePoints := pointsBase
	if isFirstOfDay {
		basePoints += pointsFirstOfDay
	}
	if req.PhotoKey != "" {
		basePoints += pointsWithPhoto
	}

	multiplier, weeklyBonus, err := s.updateWeeklyStreak(ctx, userID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to update weekly streak: %w", err)
	}

	subtotalCheckin := int(float64(basePoints) * multiplier)
	totalPoints := subtotalCheckin + weeklyBonus

	photoExpiresAt := now.Add(photoViewDuration)
	photoViewURL, err := s.storage.PresignedGetURL(ctx, req.PhotoKey, photoViewDuration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate photo view URL: %w", err)
	}
	photoURLStr := photoViewURL

	checkin := &domain.CheckIn{
		CheckInID:        uuid.New(),
		UserID:           userID,
		PlaceID:          placeID,
		PhotoURL:         &photoURLStr,
		PhotoExpiresAt:   &photoExpiresAt,
		PhotoIsPermanent: req.PhotoIsPermanent,
		GpsValid:         true,
		ValidationPassed: true,
		PointsAwarded:    totalPoints,
		IsFirstOfDay:     isFirstOfDay,
	}

	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("failed to save check-in: %w", err)
	}

	if err := s.checkinRepo.UpdateUserPoints(ctx, userID, totalPoints, checkin.CheckInID); err != nil {
		return nil, fmt.Errorf("failed to update points: %w", err)
	}

	// Non-fatal: complete any accepted plans for this user+place; plan bonuses are
	// scaled by the same weekly streak multiplier and awarded directly by the plan repo.
	planBonus := 0
	if bonus, err := s.planRepo.CompleteMatchingPlans(ctx, userID, placeID, multiplier); err != nil {
		fmt.Printf("warning: plan completion failed for user %s at place %s: %v\n", userID, placeID, err)
	} else {
		planBonus = bonus
	}

	breakdown := domain.PointsBreakdown{
		Base:            basePoints,
		Multiplier:      multiplier,
		SubtotalCheckin: subtotalCheckin,
		WeeklyBonus:     weeklyBonus,
		PlanBonus:       planBonus,
		Total:           totalPoints + planBonus,
	}

	return &domain.CheckInResult{
		CheckIn:         *checkin,
		PointsAwarded:   totalPoints,
		IsFirstOfDay:    isFirstOfDay,
		PhotoViewURL:    photoViewURL,
		PointsBreakdown: breakdown,
	}, nil
}

// SetPermanent marks or unmarks a check-in's photo as permanent. Only the owner
// may change it, and only while the place's event is still active (a check-in
// there in the last 4 hours) — once the event closes the decision is final.
func (s *CheckInService) SetPermanent(ctx context.Context, userID, checkinID uuid.UUID, permanent bool) (*domain.CheckIn, error) {
	checkin, err := s.checkinRepo.GetByID(ctx, checkinID)
	if err != nil {
		if errors.Is(err, repo.ErrCheckInNotFound) {
			return nil, ErrCheckInNotFound
		}
		return nil, err
	}

	if checkin.UserID != userID {
		return nil, ErrNotCheckInOwner
	}

	activeCount, err := s.checkinRepo.GetActiveCheckinsCount(ctx, checkin.PlaceID)
	if err != nil {
		return nil, err
	}
	if activeCount == 0 {
		return nil, ErrEventClosed
	}

	if err := s.checkinRepo.SetPhotoPermanent(ctx, checkinID, permanent); err != nil {
		return nil, err
	}

	checkin.PhotoIsPermanent = permanent
	return checkin, nil
}
