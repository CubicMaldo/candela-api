package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/testhelper"
)

type privacyTestUser struct {
	ID           uuid.UUID
	Username     string
	Email        string
	PrivacyMode  string
	RankingOptIn bool
	IsProtected  bool
}

func insertPrivacyTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, u privacyTestUser) uuid.UUID {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	if u.Username == "" {
		u.Username = fmt.Sprintf("u_%s", u.ID.String()[:8])
	}
	if u.Email == "" {
		u.Email = fmt.Sprintf("%s@test.com", u.Username)
	}
	if u.PrivacyMode == "" {
		u.PrivacyMode = "PUBLIC"
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO users (
			user_id, email, username, password_hash, birth_date,
			is_protected, account_status, privacy_mode, ranking_opt_in,
			points_total, points_season, badge_level
		) VALUES ($1, $2, $3, 'hash123', '2000-01-01', $4, 'ACTIVE', $5, $6, 500, 100, 'Callejero')
	`, u.ID, u.Email, u.Username, u.IsProtected, u.PrivacyMode, u.RankingOptIn)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM plans WHERE creator_user_id = $1 OR target_user_id = $1`, u.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM friendships WHERE user_id_a = $1 OR user_id_b = $1`, u.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM point_transactions WHERE user_id = $1`, u.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM check_ins WHERE user_id = $1`, u.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE user_id = $1`, u.ID)
	})

	return u.ID
}

func makeFriends(t *testing.T, ctx context.Context, pool *pgxpool.Pool, u1, u2 uuid.UUID) {
	idA, idB := u1, u2
	if idA.String() > idB.String() {
		idA, idB = idB, idA
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO friendships (user_id_a, user_id_b, requester_id, status, created_at)
		VALUES ($1, $2, $3, 'accepted', NOW())
		ON CONFLICT (user_id_a, user_id_b) DO UPDATE SET status = 'accepted'
	`, idA, idB, u1)
	if err != nil {
		t.Fatalf("failed to make friends: %v", err)
	}
}

// 1. Cuentas protegidas (menores): Intentar cambiar a PUBLIC siendo menor/protegido falla con ErrProtectedCannotBePublic.
func TestUpdatePrivacy_ProtectedMinorCannotBePublic(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	ur := repo.NewUserRepository(pool)
	fr := repo.NewFriendshipRepository(pool)
	rr := repo.NewRefreshTokenRepository(pool)
	userSvc := NewUserService(ur, fr, rr, mockStore, rdb)

	minorID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		IsProtected: true,
		PrivacyMode: "FRIENDS",
	})

	// Attempting to set PUBLIC while isProtected == true must fail
	err := userSvc.UpdatePrivacy(ctx, minorID, true, "PUBLIC")
	if !errors.Is(err, ErrProtectedCannotBePublic) {
		t.Fatalf("expected ErrProtectedCannotBePublic, got: %v", err)
	}

	// Updating to PRIVATE must succeed
	err = userSvc.UpdatePrivacy(ctx, minorID, true, "PRIVATE")
	if err != nil {
		t.Fatalf("expected update to PRIVATE to succeed, got: %v", err)
	}
}

// 2. Visibilidad de perfiles (PUBLIC, FRIENDS, PRIVATE).
func TestGetProfile_PrivacyMatrix(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	ur := repo.NewUserRepository(pool)
	fr := repo.NewFriendshipRepository(pool)
	rr := repo.NewRefreshTokenRepository(pool)
	userSvc := NewUserService(ur, fr, rr, mockStore, rdb)

	strangerID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{PrivacyMode: "PUBLIC"})
	friendID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{PrivacyMode: "PUBLIC"})

	// --- Case A: PRIVATE Profile ---
	targetPrivateID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:    "target_priv",
		PrivacyMode: "PRIVATE",
	})
	makeFriends(t, ctx, pool, targetPrivateID, friendID)

	// Self: gets FullUserProfile
	resSelf, err := userSvc.GetProfile(ctx, targetPrivateID, "target_priv")
	if err != nil {
		t.Fatalf("self viewing private profile failed: %v", err)
	}
	if resSelf.Full == nil || resSelf.Partial != nil {
		t.Errorf("expected Full profile for self on PRIVATE profile")
	}

	// Friend: gets PartialUserProfile
	resFriend, err := userSvc.GetProfile(ctx, friendID, "target_priv")
	if err != nil {
		t.Fatalf("friend viewing private profile failed: %v", err)
	}
	if resFriend.Partial == nil || resFriend.Full != nil {
		t.Errorf("expected Partial profile for friend on PRIVATE profile")
	}

	// Stranger: rejected with ErrProfilePrivate
	_, err = userSvc.GetProfile(ctx, strangerID, "target_priv")
	if !errors.Is(err, ErrProfilePrivate) {
		t.Fatalf("expected ErrProfilePrivate for stranger on PRIVATE profile, got: %v", err)
	}

	// --- Case B: FRIENDS Profile ---
	targetFriendsID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:    "target_frnd",
		PrivacyMode: "FRIENDS",
	})
	makeFriends(t, ctx, pool, targetFriendsID, friendID)

	// Self: FullUserProfile
	resFSelf, err := userSvc.GetProfile(ctx, targetFriendsID, "target_frnd")
	if err != nil || resFSelf.Full == nil {
		t.Fatalf("expected Full profile for self on FRIENDS profile")
	}

	// Friend: FullUserProfile
	resFFriend, err := userSvc.GetProfile(ctx, friendID, "target_frnd")
	if err != nil || resFFriend.Full == nil {
		t.Fatalf("expected Full profile for friend on FRIENDS profile")
	}

	// Stranger: PartialUserProfile
	resFStranger, err := userSvc.GetProfile(ctx, strangerID, "target_frnd")
	if err != nil || resFStranger.Partial == nil || resFStranger.Full != nil {
		t.Fatalf("expected Partial profile for stranger on FRIENDS profile, got: %v", err)
	}

	// --- Case C: PUBLIC Profile ---
	_ = insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:    "target_pub",
		PrivacyMode: "PUBLIC",
	})
	resPublic, err := userSvc.GetProfile(ctx, strangerID, "target_pub")
	if err != nil || resPublic.Full == nil {
		t.Fatalf("expected Full profile for stranger on PUBLIC profile, got: %v", err)
	}
}

// 3. Ocultamiento de puntos: Si ranking_opt_in = false, points_total es nil y se omite en el JSON.
func TestGetProfile_PointsHiddenWhenOptOut(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	ur := repo.NewUserRepository(pool)
	fr := repo.NewFriendshipRepository(pool)
	rr := repo.NewRefreshTokenRepository(pool)
	userSvc := NewUserService(ur, fr, rr, mockStore, rdb)

	// User with ranking_opt_in = false
	optOutID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:     "optout_user",
		PrivacyMode:  "PUBLIC",
		RankingOptIn: false,
	})

	res, err := userSvc.GetProfile(ctx, optOutID, "optout_user")
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if res.Full == nil {
		t.Fatal("expected FullUserProfile")
	}
	if res.Full.PointsTotal != nil {
		t.Errorf("expected PointsTotal to be nil when ranking_opt_in is false, got %d", *res.Full.PointsTotal)
	}

	// Verify JSON serialization omits points_total entirely
	jsonBytes, err := json.Marshal(res.Full)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	jsonStr := string(jsonBytes)
	if strings.Contains(jsonStr, "points_total") {
		t.Errorf("expected points_total to be omitted from JSON, got: %s", jsonStr)
	}

	// User with ranking_opt_in = true
	optInID := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:     "optin_user",
		PrivacyMode:  "PUBLIC",
		RankingOptIn: true,
	})
	resIn, err := userSvc.GetProfile(ctx, optInID, "optin_user")
	if err != nil {
		t.Fatalf("GetProfile failed: %v", err)
	}
	if resIn.Full.PointsTotal == nil {
		t.Errorf("expected PointsTotal to be set when ranking_opt_in is true")
	}
	jsonInBytes, _ := json.Marshal(resIn.Full)
	if !strings.Contains(string(jsonInBytes), "points_total") {
		t.Errorf("expected points_total to be included in JSON when opt in, got: %s", string(jsonInBytes))
	}
}

// 4. Aislamiento por Bloqueo:
// Si A bloqueó a B (o viceversa), no aparecen en búsquedas, no pueden ver perfiles (ErrUserNotVisible),
// no pueden proponerse planes ni ver check-ins mutuos.
func TestBlockIsolation(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	ur := repo.NewUserRepository(pool)
	fr := repo.NewFriendshipRepository(pool)
	rr := repo.NewRefreshTokenRepository(pool)
	plr := repo.NewPlaceRepository(pool)
	feedRepo := repo.NewFeedRepository(pool)
	cr := repo.NewCheckInRepository(pool, "America/Bogota")

	userSvc := NewUserService(ur, fr, rr, mockStore, rdb)
	planSvc := NewPlanService(repo.NewPlanRepository(pool), fr, plr)

	userA := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:    "block_alpha",
		PrivacyMode: "PUBLIC",
	})
	userB := insertPrivacyTestUser(t, ctx, pool, privacyTestUser{
		Username:    "block_bravo",
		PrivacyMode: "PUBLIC",
	})

	// User A blocks User B
	err := userSvc.Block(ctx, userA, userB)
	if err != nil {
		t.Fatalf("Block failed: %v", err)
	}

	// 4.1 Profiles: Neither A nor B can view each other's profile -> ErrUserNotVisible
	_, err = userSvc.GetProfile(ctx, userA, "block_bravo")
	if !errors.Is(err, ErrUserNotVisible) {
		t.Fatalf("expected ErrUserNotVisible for blocker viewing blocked, got: %v", err)
	}
	_, err = userSvc.GetProfile(ctx, userB, "block_alpha")
	if !errors.Is(err, ErrUserNotVisible) {
		t.Fatalf("expected ErrUserNotVisible for blocked viewing blocker, got: %v", err)
	}

	// 4.2 Search: Neither appears in the other's search results
	resultsForA, err := userSvc.Search(ctx, userA, "block_bravo")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	for _, entry := range resultsForA {
		if entry.UserID == userB {
			t.Errorf("blocked user B should not appear in search results for user A")
		}
	}

	resultsForB, err := userSvc.Search(ctx, userB, "block_alpha")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	for _, entry := range resultsForB {
		if entry.UserID == userA {
			t.Errorf("blocker user A should not appear in search results for user B")
		}
	}

	// 4.3 Plans: Neither can propose plans to the other -> ErrUserNotVisible
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")
	_, err = planSvc.Create(ctx, userA, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userB,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrUserNotVisible) {
		t.Fatalf("expected ErrUserNotVisible when user A creates plan with blocked user B, got: %v", err)
	}

	_, err = planSvc.Create(ctx, userB, domain.CreatePlanRequest{
		PlaceID:       placeID,
		TargetUserID:  &userA,
		DeadlineHours: 24,
	})
	if !errors.Is(err, ErrUserNotVisible) {
		t.Fatalf("expected ErrUserNotVisible when user B creates plan with blocker user A, got: %v", err)
	}

	// 4.4 Feed: Check-ins by blocked users are excluded
	// Create check-in for User B
	checkinB := &domain.CheckIn{
		CheckInID:        uuid.New(),
		UserID:           userB,
		PlaceID:          placeID,
		Lat:              11.0110,
		Lon:              -74.8080,
		GpsValid:         true,
		ValidationPassed: true,
	}
	if err := cr.Create(ctx, checkinB); err != nil {
		t.Fatalf("failed to create check-in for user B: %v", err)
	}

	feedA, err := feedRepo.GetFeedCheckIns(ctx, userA, 50, nil)
	if err != nil {
		t.Fatalf("GetFeedCheckIns failed: %v", err)
	}
	for _, ev := range feedA {
		if ev.User.UserID == userB {
			t.Errorf("User B checkin should not appear in User A's feed because they are blocked")
		}
	}
}
