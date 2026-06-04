package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

var (
	ErrInvalidDeadlineHours    = errors.New("deadline_hours must be 24, 48, or 72")
	ErrCannotChallengeSelf     = errors.New("you cannot challenge yourself")
	ErrTargetNotFriend         = errors.New("you can only challenge accepted friends")
	ErrPlaceNotActive          = errors.New("this place is not available for challenges")
	ErrDuplicateChallenge      = errors.New("an active challenge already exists for this place with this friend")
	ErrChallengeNotFound       = errors.New("challenge not found")
	ErrNotChallengeParticipant = errors.New("challenge not found")
	ErrNotChallengeTarget      = errors.New("you are not the target of this challenge")
	ErrChallengeNotPending     = errors.New("challenge is no longer pending")
	ErrChallengeExpired        = errors.New("challenge has expired")
)

type ChallengeService struct {
	challengeRepo  *repo.ChallengeRepository
	friendshipRepo *repo.FriendshipRepository
	placeRepo      *repo.PlaceRepository
}

func NewChallengeService(cr *repo.ChallengeRepository, fr *repo.FriendshipRepository, pr *repo.PlaceRepository) *ChallengeService {
	return &ChallengeService{challengeRepo: cr, friendshipRepo: fr, placeRepo: pr}
}

func (s *ChallengeService) Create(ctx context.Context, creatorID uuid.UUID, req domain.CreateChallengeRequest) (*domain.Challenge, error) {
	if creatorID == req.TargetUserID {
		return nil, ErrCannotChallengeSelf
	}

	valid := req.DeadlineHours == 24 || req.DeadlineHours == 48 || req.DeadlineHours == 72
	if !valid {
		return nil, ErrInvalidDeadlineHours
	}

	blocked, err := s.friendshipRepo.IsBlocked(ctx, creatorID, req.TargetUserID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrUserNotVisible
	}

	areFriends, err := s.friendshipRepo.AreFriends(ctx, creatorID, req.TargetUserID)
	if err != nil {
		return nil, err
	}
	if !areFriends {
		return nil, ErrTargetNotFriend
	}

	active, err := s.placeRepo.IsActive(ctx, req.PlaceID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrPlaceNotActive
	}

	duplicate, err := s.challengeRepo.HasActiveChallenge(ctx, creatorID, req.TargetUserID, req.PlaceID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, ErrDuplicateChallenge
	}

	challenge := &domain.Challenge{
		ChallengeID:   uuid.New(),
		CreatorUserID: creatorID,
		TargetUserID:  req.TargetUserID,
		PlaceID:       req.PlaceID,
		DeadlineHours: req.DeadlineHours,
		Status:        "PENDING",
		Message:       req.Message,
		DeadlineAt:    time.Now().UTC().Add(time.Duration(req.DeadlineHours) * time.Hour),
	}

	if err := s.challengeRepo.Create(ctx, challenge); err != nil {
		return nil, err
	}
	return challenge, nil
}

func (s *ChallengeService) Accept(ctx context.Context, requesterID, challengeID uuid.UUID) (*domain.ChallengeDetail, error) {
	detail, err := s.challengeRepo.GetByID(ctx, challengeID)
	if err != nil {
		if errors.Is(err, repo.ErrChallengeNotFound) {
			return nil, ErrChallengeNotFound
		}
		return nil, err
	}

	if detail.TargetUserID != requesterID {
		return nil, ErrNotChallengeTarget
	}
	if detail.Status != "PENDING" {
		return nil, ErrChallengeNotPending
	}
	if time.Now().UTC().After(detail.DeadlineAt) {
		return nil, ErrChallengeExpired
	}

	if err := s.challengeRepo.Accept(ctx, challengeID, requesterID); err != nil {
		return nil, err
	}
	detail.Status = "ACCEPTED"
	return detail, nil
}

func (s *ChallengeService) Reject(ctx context.Context, requesterID, challengeID uuid.UUID) error {
	detail, err := s.challengeRepo.GetByID(ctx, challengeID)
	if err != nil {
		if errors.Is(err, repo.ErrChallengeNotFound) {
			return ErrChallengeNotFound
		}
		return err
	}

	if detail.TargetUserID != requesterID {
		return ErrNotChallengeTarget
	}
	if detail.Status != "PENDING" {
		return ErrChallengeNotPending
	}

	return s.challengeRepo.Reject(ctx, challengeID, requesterID)
}

func (s *ChallengeService) GetByID(ctx context.Context, requesterID, challengeID uuid.UUID) (*domain.ChallengeDetail, error) {
	detail, err := s.challengeRepo.GetByID(ctx, challengeID)
	if err != nil {
		if errors.Is(err, repo.ErrChallengeNotFound) {
			return nil, ErrChallengeNotFound
		}
		return nil, err
	}

	if detail.CreatorUserID != requesterID && detail.TargetUserID != requesterID {
		return nil, ErrNotChallengeParticipant
	}
	return detail, nil
}

func (s *ChallengeService) GetIncoming(ctx context.Context, userID uuid.UUID) ([]domain.ChallengeDetail, error) {
	if err := s.challengeRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}
	results, err := s.challengeRepo.GetIncoming(ctx, userID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.ChallengeDetail{}
	}
	return results, nil
}

func (s *ChallengeService) GetOutgoing(ctx context.Context, userID uuid.UUID) ([]domain.ChallengeDetail, error) {
	if err := s.challengeRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}
	results, err := s.challengeRepo.GetOutgoing(ctx, userID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.ChallengeDetail{}
	}
	return results, nil
}
