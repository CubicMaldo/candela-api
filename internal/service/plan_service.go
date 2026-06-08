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
	ErrInvalidDeadlineHours = errors.New("deadline_hours must be 24, 48, or 72")
	ErrCannotPlanWithSelf   = errors.New("No te puedes proponer planes a ti mismo.")
	ErrTargetNotFriend      = errors.New("Solo puedes proponer planes a tus amigos.")
	ErrPlaceNotActive       = errors.New("Este lugar no está disponible en este momento.")
	ErrDuplicatePlan        = errors.New("Ya tienes un plan activo con esta persona para este lugar.")
	ErrPlanNotFound         = errors.New("Plan no encontrado.")
	ErrNotPlanParticipant   = errors.New("Plan no encontrado.")
	ErrNotPlanTarget        = errors.New("you are not the target of this plan")
	ErrPlanNotPending       = errors.New("plan is no longer pending")
	ErrPlanExpired          = errors.New("plan has expired")
)

type PlanService struct {
	planRepo       *repo.PlanRepository
	friendshipRepo *repo.FriendshipRepository
	placeRepo      *repo.PlaceRepository
}

func NewPlanService(pr *repo.PlanRepository, fr *repo.FriendshipRepository, plr *repo.PlaceRepository) *PlanService {
	return &PlanService{planRepo: pr, friendshipRepo: fr, placeRepo: plr}
}

func (s *PlanService) Create(ctx context.Context, creatorID uuid.UUID, req domain.CreatePlanRequest) (*domain.Plan, error) {
	if creatorID == req.TargetUserID {
		return nil, ErrCannotPlanWithSelf
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

	duplicate, err := s.planRepo.HasActivePlan(ctx, creatorID, req.TargetUserID, req.PlaceID)
	if err != nil {
		return nil, err
	}
	if duplicate {
		return nil, ErrDuplicatePlan
	}

	plan := &domain.Plan{
		PlanID:        uuid.New(),
		CreatorUserID: creatorID,
		TargetUserID:  req.TargetUserID,
		PlaceID:       req.PlaceID,
		DeadlineHours: req.DeadlineHours,
		Status:        domain.PlanPending,
		Message:       req.Message,
		DeadlineAt:    time.Now().UTC().Add(time.Duration(req.DeadlineHours) * time.Hour),
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func (s *PlanService) Accept(ctx context.Context, requesterID, planID uuid.UUID) (*domain.PlanDetail, error) {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	if detail.TargetUserID != requesterID {
		return nil, ErrNotPlanTarget
	}
	if detail.Status != domain.PlanPending {
		return nil, ErrPlanNotPending
	}
	if time.Now().UTC().After(detail.DeadlineAt) {
		return nil, ErrPlanExpired
	}

	if err := s.planRepo.Accept(ctx, planID, requesterID); err != nil {
		return nil, err
	}
	detail.Status = domain.PlanAccepted
	return detail, nil
}

func (s *PlanService) Decline(ctx context.Context, requesterID, planID uuid.UUID) error {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return ErrPlanNotFound
		}
		return err
	}

	if detail.TargetUserID != requesterID {
		return ErrNotPlanTarget
	}
	if detail.Status != domain.PlanPending {
		return ErrPlanNotPending
	}

	return s.planRepo.Decline(ctx, planID, requesterID)
}

func (s *PlanService) GetByID(ctx context.Context, requesterID, planID uuid.UUID) (*domain.PlanDetail, error) {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	if detail.CreatorUserID != requesterID && detail.TargetUserID != requesterID {
		return nil, ErrNotPlanParticipant
	}
	return detail, nil
}

func (s *PlanService) GetIncoming(ctx context.Context, userID uuid.UUID) ([]domain.PlanDetail, error) {
	if err := s.planRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}
	results, err := s.planRepo.GetIncoming(ctx, userID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.PlanDetail{}
	}
	return results, nil
}

func (s *PlanService) GetOutgoing(ctx context.Context, userID uuid.UUID) ([]domain.PlanDetail, error) {
	if err := s.planRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}
	results, err := s.planRepo.GetOutgoing(ctx, userID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.PlanDetail{}
	}
	return results, nil
}
