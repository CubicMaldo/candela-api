package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/testhelper"
)

// 4.1 Restricciones de creación de planes 1:1 (INVITE)
// Falla si el destinatario no es amigo, si está bloqueado o si ya existe un plan activo para ese lugar.
func TestPlanService_Create_InviteRestrictions(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	fr := repo.NewFriendshipRepository(pool)
	plr := repo.NewPlaceRepository(pool)
	pr := repo.NewPlanRepository(pool)
	planSvc := NewPlanService(pr, fr, plr)

	userA := createTestUser(t, ctx, pool, false)
	userNotFriend := createTestUser(t, ctx, pool, false)
	userBlocked := createTestUser(t, ctx, pool, false)
	userFriend := createTestUser(t, ctx, pool, false)

	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza

	// Setup friendships
	makeFriends(t, ctx, pool, userA, userFriend)
	makeFriends(t, ctx, pool, userA, userBlocked)
	// Block userBlocked
	ur := repo.NewUserRepository(pool)
	rr := repo.NewRefreshTokenRepository(pool)
	userSvc := NewUserService(ur, fr, rr, testhelper.NewMockStorage(), rdb)
	if err := userSvc.Block(ctx, userA, userBlocked); err != nil {
		t.Fatalf("Block failed: %v", err)
	}

	// 1. Fails if target is not a friend
	_, err := planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userNotFriend,
		Visibility:    domain.PlanVisibilityInvite,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrTargetNotFriend) {
		t.Fatalf("expected ErrTargetNotFriend, got: %v", err)
	}

	// 2. Fails if target is blocked
	_, err = planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userBlocked,
		Visibility:    domain.PlanVisibilityInvite,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrUserNotVisible) {
		t.Fatalf("expected ErrUserNotVisible for blocked target, got: %v", err)
	}

	// 3. First plan with valid friend succeeds
	plan1, err := planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userFriend,
		Visibility:    domain.PlanVisibilityInvite,
		DeadlineHours: 24,
	})
	if err != nil {
		t.Fatalf("valid plan creation failed: %v", err)
	}
	if plan1.PlanID == uuid.Nil {
		t.Fatal("expected non-nil plan ID")
	}

	// 4. Duplicate plan with the same friend for the same place fails with ErrDuplicatePlan
	_, err = planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userFriend,
		Visibility:    domain.PlanVisibilityInvite,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrDuplicatePlan) {
		t.Fatalf("expected ErrDuplicatePlan, got: %v", err)
	}
}

// 4.2 Restricciones de creación de planes abiertos (PUBLIC / FRIENDS)
// Probar que no admiten target_user_id y que solo se permite un plan abierto activo por lugar por usuario.
func TestPlanService_Create_OpenPlanRestrictions(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	fr := repo.NewFriendshipRepository(pool)
	plr := repo.NewPlaceRepository(pool)
	pr := repo.NewPlanRepository(pool)
	planSvc := NewPlanService(pr, fr, plr)

	userA := createTestUser(t, ctx, pool, false)
	targetUser := createTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	// 1. Open plans must not provide target_user_id -> ErrTargetNotAllowed
	_, err := planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &targetUser,
		Visibility:    domain.PlanVisibilityPublic,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrTargetNotAllowed) {
		t.Fatalf("expected ErrTargetNotAllowed when target provided for PUBLIC plan, got: %v", err)
	}

	_, err = planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &targetUser,
		Visibility:    domain.PlanVisibilityFriends,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrTargetNotAllowed) {
		t.Fatalf("expected ErrTargetNotAllowed when target provided for FRIENDS plan, got: %v", err)
	}

	// 2. Open plan without target succeeds
	openPlan, err := planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		Visibility:    domain.PlanVisibilityPublic,
		DeadlineHours: 24,
	})
	if err != nil {
		t.Fatalf("creating open plan failed: %v", err)
	}
	if openPlan.PlanID == uuid.Nil {
		t.Fatal("expected non-nil plan ID")
	}

	// 3. Second open plan by the same creator for the same place fails with ErrDuplicateOpenPlan
	_, err = planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		Visibility:    domain.PlanVisibilityFriends,
		DeadlineHours: 48,
	})
	if !errors.Is(err, ErrDuplicateOpenPlan) {
		t.Fatalf("expected ErrDuplicateOpenPlan, got: %v", err)
	}
}

// 4.3 Validación de plazos (DeadlineHours)
// Probar que solo se aceptan plazos válidos (24, 48 o 72 horas).
func TestPlanService_Create_DeadlineValidation(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	fr := repo.NewFriendshipRepository(pool)
	plr := repo.NewPlaceRepository(pool)
	pr := repo.NewPlanRepository(pool)
	planSvc := NewPlanService(pr, fr, plr)

	userA := createTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	invalidHours := []int{0, 1, 12, 36, 50, 96, -24}
	for _, h := range invalidHours {
		_, err := planSvc.Create(ctx, userA, domain.CreatePlanRequest{
			PlaceID:       placeID,
			Visibility:    domain.PlanVisibilityPublic,
			DeadlineHours: h,
		})
		if !errors.Is(err, ErrInvalidDeadlineHours) {
			t.Errorf("expected ErrInvalidDeadlineHours for deadline %d hours, got: %v", h, err)
		}
	}

	// Valid hours: 24, 48, 72
	validHours := []struct {
		hours   int
		placeID uuid.UUID
	}{
		{24, uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")},
		{48, uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000004")},
		{72, uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000005")},
	}
	for _, tc := range validHours {
		p, err := planSvc.Create(ctx, userA, domain.CreatePlanRequest{
			PlaceID:       tc.placeID,
			Visibility:    domain.PlanVisibilityPublic,
			DeadlineHours: tc.hours,
		})
		if err != nil {
			t.Errorf("expected valid plan creation for %d hours, got: %v", tc.hours, err)
		}
		if p.DeadlineHours != tc.hours {
			t.Errorf("expected DeadlineHours %d, got %d", tc.hours, p.DeadlineHours)
		}
	}
}

// 4.4 Completitud automática por Check-in
// Probar que al confirmar un check-in en el lugar del plan, se llama a CompleteMatchingPlans,
// cambiando el estado a COMPLETED y sumando el bono escalado por el multiplicador de racha.
func TestPlanService_CheckInAutoCompletion(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	fr := repo.NewFriendshipRepository(pool)
	plr := repo.NewPlaceRepository(pool)
	pr := repo.NewPlanRepository(pool)
	cr := repo.NewCheckInRepository(pool, "America/Bogota")

	planSvc := NewPlanService(pr, fr, plr)
	checkInSvc := NewCheckInService(cr, pr, mockStore, rdb)

	userCreator := createTestUser(t, ctx, pool, false)
	userGuest := createTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza: 11.0110, -74.8080

	makeFriends(t, ctx, pool, userCreator, userGuest)

	// 1. Creator proposes INVITE plan to Guest
	msg := "Vamos por un café!"
	plan, err := planSvc.Create(ctx, userCreator, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userGuest,
		Visibility:    domain.PlanVisibilityInvite,
		DeadlineHours: 24,
		Message:       &msg,
	})
	if err != nil {
		t.Fatalf("failed to create plan: %v", err)
	}

	// 2. Guest accepts the plan
	_, err = planSvc.Accept(ctx, userGuest, plan.PlanID)
	if err != nil {
		t.Fatalf("Guest Accept failed: %v", err)
	}

	// Verify plan status is ACCEPTED
	planDetail, err := pr.GetByID(ctx, plan.PlanID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if planDetail.Status != domain.PlanAccepted {
		t.Fatalf("expected plan status ACCEPTED, got: %s", planDetail.Status)
	}

	// 3. UserCreator checks in at the place
	initResp1, err := checkInSvc.Initiate(ctx, userCreator, placeID, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("Creator Initiate check-in failed: %v", err)
	}
	res1, err := checkInSvc.Confirm(ctx, userCreator, placeID, domain.CheckInConfirmRequest{
		CheckInNonce: initResp1.CheckInNonce,
		PhotoKey:     initResp1.PhotoKey,
	})
	if err != nil {
		t.Fatalf("Creator Confirm check-in failed: %v", err)
	}

	// Creator earns plan bonus (base 10 * multiplier 1.0 = 10)
	if res1.PointsBreakdown.PlanBonus <= 0 {
		t.Errorf("expected PlanBonus > 0 for creator, got %d", res1.PointsBreakdown.PlanBonus)
	}

	// Plan should still be ACCEPTED because only 1 participant completed so far
	planDetail, _ = pr.GetByID(ctx, plan.PlanID)
	if planDetail.Status != domain.PlanAccepted {
		t.Fatalf("expected plan status still ACCEPTED with 1 attendee, got: %s", planDetail.Status)
	}

	// 4. UserGuest checks in at the place
	initResp2, err := checkInSvc.Initiate(ctx, userGuest, placeID, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("Guest Initiate check-in failed: %v", err)
	}
	res2, err := checkInSvc.Confirm(ctx, userGuest, placeID, domain.CheckInConfirmRequest{
		CheckInNonce: initResp2.CheckInNonce,
		PhotoKey:     initResp2.PhotoKey,
	})
	if err != nil {
		t.Fatalf("Guest Confirm check-in failed: %v", err)
	}

	// Guest also earns plan bonus
	if res2.PointsBreakdown.PlanBonus <= 0 {
		t.Errorf("expected PlanBonus > 0 for guest, got %d", res2.PointsBreakdown.PlanBonus)
	}

	// 5. Now that >= 2 participants completed check-in, the plan MUST be COMPLETED!
	planDetail, err = pr.GetByID(ctx, plan.PlanID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if planDetail.Status != domain.PlanCompleted {
		t.Errorf("expected plan status COMPLETED after both checked in, got: %s", planDetail.Status)
	}
	if planDetail.CompletedAt == nil || planDetail.CompletedAt.IsZero() {
		t.Errorf("expected plan CompletedAt to be set")
	}
}
