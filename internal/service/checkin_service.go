package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	ErrPlaceRestricted     = errors.New("Este lugar no está disponible en este momento.")
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

	// exifSkewTolerance is how far the client-reported capture time may drift
	// from the server clock before the photo is flagged. It is recorded, not
	// enforced — see checkInSession.
	exifSkewTolerance = 5 * time.Minute
)

// checkInSession is the server-side state bound to a check-in nonce. It holds
// everything Confirm needs that must not be taken from the client: the
// server-validated GPS fix and the moment the session was opened.
//
// This session — created only after a successful geofence check and expiring
// with the nonce after 90 seconds — is what makes a check-in "live". The
// client's EXIF timestamp is not: it comes from the device clock, so a phone
// a few minutes out of sync could never check in, while anyone crafting the
// request could set it to whatever passes. It is now recorded as a fraud
// signal (TimestampValid) instead of gating the flow.
type checkInSession struct {
	UserID   uuid.UUID `json:"user_id"`
	PlaceID  uuid.UUID `json:"place_id"`
	PhotoKey string    `json:"photo_key"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	IssuedAt time.Time `json:"issued_at"`
}

type CheckInService struct {
	checkinRepo *repo.CheckInRepository
	planRepo    *repo.PlanRepository
	storage     storage.ObjectStorage
	redis       *redis.Client
}

func NewCheckInService(cr *repo.CheckInRepository, pr *repo.PlanRepository, st storage.ObjectStorage, rdb *redis.Client) *CheckInService {
	return &CheckInService{checkinRepo: cr, planRepo: pr, storage: st, redis: rdb}
}

func checkInNonceKey(nonce string) string { return "checkin:nonce:" + nonce }

// discardPhoto removes an uploaded object that will never be referenced by a
// check-in row. Best effort: the check-in has already failed, so a failed
// cleanup is logged for the sweeper to pick up rather than surfaced.
func (s *CheckInService) discardPhoto(ctx context.Context, key string) {
	if key == "" {
		return
	}
	if err := s.storage.Delete(ctx, key); err != nil {
		log.Printf("checkin: could not discard orphaned photo %s: %v", key, err)
	}
}

// Initiate validates GPS and returns a presigned upload URL. The resulting
// session is stored in Redis for 90 seconds, binding the upload — and the
// server's own view of where the user was — to this attempt.
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
			return nil, ErrPlaceRestricted
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

	session := checkInSession{
		UserID:   userID,
		PlaceID:  placeID,
		PhotoKey: photoKey,
		Lat:      req.Lat,
		Lon:      req.Lon,
		IssuedAt: time.Now().UTC(),
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("failed to encode check-in session: %w", err)
	}
	if err := s.redis.Set(ctx, checkInNonceKey(nonce), encoded, nonceTTL).Err(); err != nil {
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
//
// Once the nonce is consumed, any failure abandons an already-uploaded photo,
// so every error path past that point deletes the object before returning.
func (s *CheckInService) Confirm(
	ctx context.Context,
	userID, placeID uuid.UUID,
	req domain.CheckInConfirmRequest,
) (*domain.CheckInResult, error) {

	nonceKey := checkInNonceKey(req.CheckInNonce)
	raw, err := s.redis.Get(ctx, nonceKey).Result()
	if err == redis.Nil {
		return nil, ErrInvalidCheckInNonce
	}
	if err != nil {
		return nil, fmt.Errorf("failed to validate check-in session: %w", err)
	}

	var session checkInSession
	if err := json.Unmarshal([]byte(raw), &session); err != nil {
		return nil, ErrInvalidCheckInNonce
	}
	if session.UserID != userID || session.PlaceID != placeID || session.PhotoKey != req.PhotoKey {
		return nil, ErrInvalidCheckInNonce
	}

	// Single-use: delete immediately after validation.
	s.redis.Del(ctx, nonceKey)

	// From here on the client has already uploaded to session.PhotoKey.
	fail := func(err error) (*domain.CheckInResult, error) {
		s.discardPhoto(ctx, session.PhotoKey)
		return nil, err
	}

	now := time.Now().UTC()

	// The client's capture time is advisory. Record whether it lines up with
	// the server clock so abuse review can spot forged or replayed photos,
	// but never block a check-in on a device whose clock is simply wrong.
	timestampValid := true
	var exifTS *time.Time
	if !req.ExifTimestamp.IsZero() {
		captured := req.ExifTimestamp.UTC()
		exifTS = &captured
		skew := now.Sub(captured)
		if skew < 0 {
			skew = -skew
		}
		timestampValid = skew <= exifSkewTolerance
	}

	isFirstOfDay, err := s.checkinRepo.IsFirstCheckInToday(ctx, userID)
	if err != nil {
		return fail(err)
	}

	basePoints := pointsBase
	if isFirstOfDay {
		basePoints += pointsFirstOfDay
	}
	if req.PhotoKey != "" {
		basePoints += pointsWithPhoto
	}

	photoExpiresAt := now.Add(photoViewDuration)
	photoViewURL, err := s.storage.PresignedGetURL(ctx, req.PhotoKey, photoViewDuration)
	if err != nil {
		return fail(fmt.Errorf("failed to generate photo view URL: %w", err))
	}
	photoURLStr := photoViewURL
	photoKey := req.PhotoKey

	checkin := &domain.CheckIn{
		CheckInID:        uuid.New(),
		UserID:           userID,
		PlaceID:          placeID,
		PhotoURL:         &photoURLStr,
		PhotoKey:         &photoKey,
		PhotoExpiresAt:   &photoExpiresAt,
		PhotoIsPermanent: req.PhotoIsPermanent,
		PhotoExifTS:      exifTS,
		TimestampValid:   timestampValid,
		// The coordinates the server itself validated at Initiate — not
		// anything the client re-sent at confirm time.
		Lat:              session.Lat,
		Lon:              session.Lon,
		GpsValid:         true,
		ValidationPassed: true,
		// Points are awarded below, once the row has claimed the day.
		PointsAwarded: 0,
		IsFirstOfDay:  isFirstOfDay,
	}

	// The check-in row is written FIRST, before any counter moves. Its unique
	// index is the only thing that can settle a race between concurrent
	// confirms; doing the streak update first meant every loser of that race
	// still bumped the user's weekly counter, inflating the multiplier with
	// check-ins that were never recorded.
	if err := s.checkinRepo.Create(ctx, checkin); err != nil {
		// Initiate already screens for same-day duplicates; reaching here means
		// concurrent requests raced past that check.
		if errors.Is(err, repo.ErrDuplicateCheckIn) {
			return fail(ErrAlreadyCheckedIn)
		}
		return fail(fmt.Errorf("failed to save check-in: %w", err))
	}

	// Past this point the check-in row owns the photo, so it is no longer an
	// orphan. A failure now leaves a claimed day the user cannot retry, so the
	// row is rolled back by hand instead.
	abort := func(err error) (*domain.CheckInResult, error) {
		if delErr := s.checkinRepo.Delete(ctx, checkin.CheckInID); delErr != nil {
			log.Printf("checkin: could not roll back check-in %s: %v", checkin.CheckInID, delErr)
		}
		s.discardPhoto(ctx, session.PhotoKey)
		return nil, err
	}

	multiplier, weeklyBonus, err := s.checkinRepo.ApplyStreak(ctx, userID,
		func(current domain.StreakState) (domain.StreakState, float64, int) {
			return domain.ApplyCheckInToStreak(current, now)
		})
	if err != nil {
		return abort(fmt.Errorf("failed to update weekly streak: %w", err))
	}

	subtotalCheckin := int(float64(basePoints) * multiplier)
	totalPoints := subtotalCheckin + weeklyBonus
	checkin.PointsAwarded = totalPoints

	if err := s.checkinRepo.SetPointsAwarded(ctx, checkin.CheckInID, totalPoints); err != nil {
		return abort(fmt.Errorf("failed to record points on the check-in: %w", err))
	}

	if err := s.checkinRepo.UpdateUserPoints(ctx, userID, totalPoints, checkin.CheckInID); err != nil {
		return nil, fmt.Errorf("failed to update points: %w", err)
	}

	// Non-fatal: complete any accepted plans for this user+place; plan bonuses are
	// scaled by the same weekly streak multiplier and awarded directly by the plan repo.
	planBonus := 0
	if bonus, err := s.planRepo.CompleteMatchingPlans(ctx, userID, placeID, multiplier); err != nil {
		log.Printf("checkin: plan completion failed for user %s at place %s: %v", userID, placeID, err)
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
