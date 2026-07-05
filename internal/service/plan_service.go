package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
)

var (
	ErrInvalidDeadlineHours = errors.New("El plazo debe ser de 24, 48 o 72 horas.")
	ErrCannotPlanWithSelf   = errors.New("No te puedes proponer planes a ti mismo.")
	ErrTargetNotFriend      = errors.New("Solo puedes proponer planes a tus amigos.")
	ErrPlaceNotActive       = errors.New("Este lugar no está disponible en este momento.")
	ErrDuplicatePlan        = errors.New("Ya tienes un plan activo con esta persona para este lugar.")
	ErrDuplicateOpenPlan    = errors.New("Ya tienes un plan abierto en este lugar.")
	ErrTargetRequired       = errors.New("Elige a quién invitar.")
	ErrTargetNotAllowed     = errors.New("Un plan abierto no lleva invitado.")
	ErrPlanNotFound         = errors.New("Plan no encontrado.")
	ErrNotPlanParticipant   = errors.New("Plan no encontrado.")
	ErrNotPlanTarget        = errors.New("Solo el destinatario puede responder este plan.")
	ErrNotPlanCreator       = errors.New("Solo quien propuso el plan puede cancelarlo.")
	ErrPlanNotPending       = errors.New("Este plan ya no está pendiente.")
	ErrPlanExpired          = errors.New("Este plan ya venció.")
	ErrPlanNotJoinable      = errors.New("Este plan no admite que te unas.")
	ErrAlreadyInPlan        = errors.New("Ya estás en este plan.")
	ErrCannotJoinOwnPlan    = errors.New("Este es tu propio plan.")
)

type PlanService struct {
	planRepo       *repo.PlanRepository
	friendshipRepo *repo.FriendshipRepository
	placeRepo      *repo.PlaceRepository
}

func NewPlanService(pr *repo.PlanRepository, fr *repo.FriendshipRepository, plr *repo.PlaceRepository) *PlanService {
	return &PlanService{planRepo: pr, friendshipRepo: fr, placeRepo: plr}
}

func (s *PlanService) Create(ctx context.Context, creatorID uuid.UUID, req domain.CreatePlanRequest) (*domain.MyPlan, error) {
	valid := req.DeadlineHours == 24 || req.DeadlineHours == 48 || req.DeadlineHours == 72
	if !valid {
		return nil, ErrInvalidDeadlineHours
	}

	// Empty visibility defaults to the classic 1:1 invited flow.
	visibility := req.Visibility
	if visibility == "" {
		visibility = domain.PlanVisibilityInvite
	}

	active, err := s.placeRepo.IsActive(ctx, req.PlaceID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrPlaceNotActive
	}

	if visibility == domain.PlanVisibilityInvite {
		if req.TargetUserID == nil {
			return nil, ErrTargetRequired
		}
		target := *req.TargetUserID
		if creatorID == target {
			return nil, ErrCannotPlanWithSelf
		}

		blocked, err := s.friendshipRepo.IsBlocked(ctx, creatorID, target)
		if err != nil {
			return nil, err
		}
		if blocked {
			return nil, ErrUserNotVisible
		}

		areFriends, err := s.friendshipRepo.AreFriends(ctx, creatorID, target)
		if err != nil {
			return nil, err
		}
		if !areFriends {
			return nil, ErrTargetNotFriend
		}

		duplicate, err := s.planRepo.HasActivePlan(ctx, creatorID, target, req.PlaceID)
		if err != nil {
			return nil, err
		}
		if duplicate {
			return nil, ErrDuplicatePlan
		}
	} else {
		// Open plan (PUBLIC/FRIENDS): no invitee, dedupe on the creator's own
		// open plan for this place.
		if req.TargetUserID != nil {
			return nil, ErrTargetNotAllowed
		}
		duplicate, err := s.planRepo.HasActiveOpenPlan(ctx, creatorID, req.PlaceID)
		if err != nil {
			return nil, err
		}
		if duplicate {
			return nil, ErrDuplicateOpenPlan
		}
	}

	plan := &domain.Plan{
		PlanID:        uuid.New(),
		CreatorUserID: creatorID,
		TargetUserID:  req.TargetUserID,
		PlaceID:       req.PlaceID,
		Visibility:    visibility,
		DeadlineHours: req.DeadlineHours,
		Status:        domain.PlanPending,
		Message:       req.Message,
		DeadlineAt:    time.Now().UTC().Add(time.Duration(req.DeadlineHours) * time.Hour),
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		return nil, err
	}
	return s.planRepo.GetMyPlanByID(ctx, plan.PlanID, creatorID)
}

// Join adds the requester as a participant of an open plan. PUBLIC plans are
// joinable by anyone; FRIENDS plans only by the creator's friends. INVITE plans
// are not openly joinable.
func (s *PlanService) Join(ctx context.Context, requesterID, planID uuid.UUID) (*domain.MyPlan, error) {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	if detail.CreatorUserID == requesterID {
		return nil, ErrCannotJoinOwnPlan
	}
	if detail.Status != domain.PlanPending {
		return nil, ErrPlanNotPending
	}
	if time.Now().UTC().After(detail.DeadlineAt) {
		return nil, ErrPlanExpired
	}

	switch detail.Visibility {
	case domain.PlanVisibilityPublic:
		// anyone may join
	case domain.PlanVisibilityFriends:
		areFriends, err := s.friendshipRepo.AreFriends(ctx, detail.CreatorUserID, requesterID)
		if err != nil {
			return nil, err
		}
		if !areFriends {
			return nil, ErrPlanNotJoinable
		}
	default:
		return nil, ErrPlanNotJoinable
	}

	if err := s.planRepo.Join(ctx, planID, requesterID); err != nil {
		if errors.Is(err, repo.ErrAlreadyJoined) {
			return nil, ErrAlreadyInPlan
		}
		return nil, err
	}
	return s.planRepo.GetMyPlanByID(ctx, planID, requesterID)
}

// Discover returns open plans near (lat, lon) the requester can join, for the
// map. radiusKM defaults are applied by the caller.
func (s *PlanService) Discover(ctx context.Context, requesterID uuid.UUID, lat, lon, radiusMeters float64) ([]domain.DiscoverPlan, error) {
	return s.planRepo.DiscoverOpenPlans(ctx, requesterID, lat, lon, radiusMeters)
}

func (s *PlanService) Accept(ctx context.Context, requesterID, planID uuid.UUID) (*domain.MyPlan, error) {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	if detail.TargetUserID == nil || *detail.TargetUserID != requesterID {
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
	return s.planRepo.GetMyPlanByID(ctx, planID, requesterID)
}

func (s *PlanService) Decline(ctx context.Context, requesterID, planID uuid.UUID) error {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return ErrPlanNotFound
		}
		return err
	}

	if detail.TargetUserID == nil || *detail.TargetUserID != requesterID {
		return ErrNotPlanTarget
	}
	if detail.Status != domain.PlanPending {
		return ErrPlanNotPending
	}

	return s.planRepo.Decline(ctx, planID, requesterID)
}

// Cancel deletes a still-pending plan. Only the creator can cancel.
func (s *PlanService) Cancel(ctx context.Context, requesterID, planID uuid.UUID) error {
	detail, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return ErrPlanNotFound
		}
		return err
	}

	if detail.CreatorUserID != requesterID {
		return ErrNotPlanCreator
	}
	if detail.Status != domain.PlanPending {
		return ErrPlanNotPending
	}

	if err := s.planRepo.Cancel(ctx, planID, requesterID); err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return ErrPlanNotFound
		}
		return err
	}
	return nil
}

func (s *PlanService) GetByID(ctx context.Context, requesterID, planID uuid.UUID) (*domain.MyPlan, error) {
	mp, err := s.planRepo.GetMyPlanByID(ctx, planID, requesterID)
	if err != nil {
		if errors.Is(err, repo.ErrPlanNotFound) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}

	// Visible to the creator or anyone who participates. MyStatus is set (from the
	// viewer's plan_participants row) for any participant, including open-plan
	// guests where Target is nil.
	if mp.MyStatus == "" && mp.Creator.UserID != requesterID {
		return nil, ErrNotPlanParticipant
	}
	return mp, nil
}

func (s *PlanService) GetIncoming(ctx context.Context, userID uuid.UUID) ([]domain.MyPlan, error) {
	if err := s.planRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}
	results, err := s.planRepo.GetIncoming(ctx, userID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.MyPlan{}
	}
	return results, nil
}

func (s *PlanService) GetOutgoing(ctx context.Context, userID uuid.UUID) ([]domain.MyPlan, error) {
	if err := s.planRepo.MarkExpiredForUser(ctx, userID); err != nil {
		return nil, err
	}
	results, err := s.planRepo.GetOutgoing(ctx, userID)
	if err != nil {
		return nil, err
	}
	if results == nil {
		results = []domain.MyPlan{}
	}
	return results, nil
}
