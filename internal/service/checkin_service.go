package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/infra/storage"
	"github.com/tuusuario/nomad-api/internal/repo"
)

var (
	ErrNotWithinGeofence   = errors.New("you must be closer to the place to check in")
	ErrAlreadyCheckedIn    = errors.New("you already checked in here today")
	ErrInvalidCheckInNonce = errors.New("check-in session expired or invalid, please try again")
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
	checkinRepo   *repo.CheckInRepository
	challengeRepo *repo.ChallengeRepository
	storage       storage.ObjectStorage
	redis         *redis.Client
}

func NewCheckInService(cr *repo.CheckInRepository, ch *repo.ChallengeRepository, st storage.ObjectStorage, rdb *redis.Client) *CheckInService {
	return &CheckInService{checkinRepo: cr, challengeRepo: ch, storage: st, redis: rdb}
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
			return nil, errors.New("this place is not available for your account type")
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

	isFirstOfDay, err := s.checkinRepo.IsFirstCheckInToday(ctx, userID)
	if err != nil {
		return nil, err
	}

	points := pointsBase
	if isFirstOfDay {
		points += pointsFirstOfDay
	}
	if req.PhotoKey != "" {
		points += pointsWithPhoto
	}

	photoExpiresAt := time.Now().UTC().Add(photoViewDuration)
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
		GpsValid:         true,
		ValidationPassed: true,
		PointsAwarded:    points,
		IsFirstOfDay:     isFirstOfDay,
	}

	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		return nil, fmt.Errorf("failed to save check-in: %w", err)
	}

	if err := s.checkinRepo.UpdateUserPoints(ctx, userID, points, checkin.CheckInID); err != nil {
		return nil, fmt.Errorf("failed to update points: %w", err)
	}

	// Non-fatal: streak failure must not roll back the check-in
	if err := s.checkinRepo.UpdateStreakAndBadge(ctx, userID, time.Now().UTC()); err != nil {
		fmt.Printf("warning: streak update failed for user %s: %v\n", userID, err)
	}

	// Non-fatal: complete any accepted challenges for this user+place
	if err := s.challengeRepo.CompleteMatchingChallenges(ctx, userID, placeID, checkin.CheckInID); err != nil {
		fmt.Printf("warning: challenge completion failed for user %s at place %s: %v\n", userID, placeID, err)
	}

	return &domain.CheckInResult{
		CheckIn:       *checkin,
		PointsAwarded: points,
		IsFirstOfDay:  isFirstOfDay,
		PhotoViewURL:  photoViewURL,
	}, nil
}
