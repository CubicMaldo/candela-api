package checkin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

// 1. Nonce single use: Confirm consumes the Redis nonce; a second call fails with ErrInvalidCheckInNonce.
func TestCheckIn_Nonce_SingleUse(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza: lon -74.8080, lat 11.0110

	initResp, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	confirmReq := domain.CheckInConfirmRequest{
		CheckInNonce: initResp.CheckInNonce,
		PhotoKey:     initResp.PhotoKey,
	}

	// First Confirm must succeed
	res, err := svc.Confirm(ctx, userID, placeID, confirmReq)
	if err != nil {
		t.Fatalf("first Confirm failed: %v", err)
	}
	if res.CheckIn.CheckInID == uuid.Nil {
		t.Fatalf("expected valid checkin ID")
	}

	// Second Confirm with the SAME nonce must fail with ErrInvalidCheckInNonce
	_, err = svc.Confirm(ctx, userID, placeID, confirmReq)
	if !errors.Is(err, service.ErrInvalidCheckInNonce) {
		t.Fatalf("expected ErrInvalidCheckInNonce on second Confirm, got: %v", err)
	}
}

// 2. Nonce expiration (> 90s): Confirm fails with ErrInvalidCheckInNonce when nonce expired.
func TestCheckIn_Nonce_Expired(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	// Store an expired session directly in Redis or delete it to simulate TTL expiration
	expiredNonce := uuid.New().String()
	expiredPhotoKey := fmt.Sprintf("checkins/%s/expired.jpg", userID.String())
	nonceKey := service.CheckInNonceKey(expiredNonce)

	session := service.CheckInSession{
		UserID:   userID,
		PlaceID:  placeID,
		PhotoKey: expiredPhotoKey,
		Lat:      11.0110,
		Lon:      -74.8080,
		IssuedAt: time.Now().UTC().Add(-100 * time.Second), // expired > 90s
	}
	encoded, _ := json.Marshal(session)
	// Set with 1 millisecond TTL and wait 10ms for expiration
	_ = rdb.Set(ctx, nonceKey, encoded, 10*time.Millisecond).Err()
	time.Sleep(20 * time.Millisecond)

	_, err := svc.Confirm(ctx, userID, placeID, domain.CheckInConfirmRequest{
		CheckInNonce: expiredNonce,
		PhotoKey:     expiredPhotoKey,
	})
	if !errors.Is(err, service.ErrInvalidCheckInNonce) {
		t.Fatalf("expected ErrInvalidCheckInNonce for expired nonce, got: %v", err)
	}
}

// 3. User & Place mismatch: Nonce of user A / place X cannot be used by user B or place Y.
func TestCheckIn_Nonce_UserMismatch_And_PlaceMismatch(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userA := testhelper.CreateTestUser(t, ctx, pool, false)
	userB := testhelper.CreateTestUser(t, ctx, pool, false)
	placeX := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza
	placeY := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000004") // Parque Cultural

	initResp, err := svc.Initiate(ctx, userA, placeX, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// Attack 1: User B tries to confirm User A's nonce
	_, err = svc.Confirm(ctx, userB, placeX, domain.CheckInConfirmRequest{
		CheckInNonce: initResp.CheckInNonce,
		PhotoKey:     initResp.PhotoKey,
	})
	if !errors.Is(err, service.ErrInvalidCheckInNonce) {
		t.Fatalf("expected ErrInvalidCheckInNonce when user B attempts to use user A's nonce, got: %v", err)
	}

	// Attack 2: User A tries to confirm at Place Y with Place X's nonce
	_, err = svc.Confirm(ctx, userA, placeY, domain.CheckInConfirmRequest{
		CheckInNonce: initResp.CheckInNonce,
		PhotoKey:     initResp.PhotoKey,
	})
	if !errors.Is(err, service.ErrInvalidCheckInNonce) {
		t.Fatalf("expected ErrInvalidCheckInNonce when confirming at Place Y with Place X's nonce, got: %v", err)
	}
}

// 4. Geofencing & Coordinate Spoofing: Coordinates saved in Initiate are persisted in CheckIn.
func TestCheckIn_Geofence_CoordsPersisted(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	expectedLat := 11.01102
	expectedLon := -74.80801

	initResp, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
		Lat: expectedLat,
		Lon: expectedLon,
	})
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// Notice: ConfirmRequest has no coordinates field at all!
	res, err := svc.Confirm(ctx, userID, placeID, domain.CheckInConfirmRequest{
		CheckInNonce: initResp.CheckInNonce,
		PhotoKey:     initResp.PhotoKey,
	})
	if err != nil {
		t.Fatalf("Confirm failed: %v", err)
	}

	// Verify coordinates on the returned CheckIn match Initiate
	if res.CheckIn.Lat != expectedLat || res.CheckIn.Lon != expectedLon {
		t.Errorf("expected coordinates (%f, %f), got (%f, %f)", expectedLat, expectedLon, res.CheckIn.Lat, res.CheckIn.Lon)
	}
}

// 5. Out of geofence: Initiate rejects immediate with ErrNotWithinGeofence.
func TestCheckIn_Geofence_OutOfRange(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza in Barranquilla

	// Bogotá coordinates (approx 1000km away)
	_, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
		Lat: 4.7110,
		Lon: -74.0721,
	})
	if !errors.Is(err, service.ErrNotWithinGeofence) {
		t.Fatalf("expected ErrNotWithinGeofence, got: %v", err)
	}
}

// 6. Idempotency & Concurrency: Two concurrent confirms for the same user & place on the same day.
func TestCheckIn_Concurrency_OnePerDay(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	// Pre-create two distinct valid nonces for this user and place
	init1, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("Initiate 1 failed: %v", err)
	}

	// To simulate the race condition where two requests pass Initiate concurrently:
	// Insert second session manually into Redis
	nonce2 := uuid.New().String()
	photoKey2 := fmt.Sprintf("checkins/%s/race2.jpg", userID.String())
	session2 := service.CheckInSession{
		UserID:   userID,
		PlaceID:  placeID,
		PhotoKey: photoKey2,
		Lat:      11.0110,
		Lon:      -74.8080,
		IssuedAt: time.Now().UTC(),
	}
	encoded2, _ := json.Marshal(session2)
	_ = rdb.Set(ctx, service.CheckInNonceKey(nonce2), encoded2, 90*time.Second).Err()

	var wg sync.WaitGroup
	successCount := 0
	duplicateCount := 0
	var mu sync.Mutex

	runConfirm := func(n, pk string) {
		defer wg.Done()
		_, cErr := svc.Confirm(ctx, userID, placeID, domain.CheckInConfirmRequest{
			CheckInNonce: n,
			PhotoKey:     pk,
		})
		mu.Lock()
		defer mu.Unlock()
		if cErr == nil {
			successCount++
		} else if errors.Is(cErr, service.ErrAlreadyCheckedIn) {
			duplicateCount++
		} else {
			t.Errorf("unexpected error in concurrent confirm: %v", cErr)
		}
	}

	wg.Add(2)
	go runConfirm(init1.CheckInNonce, init1.PhotoKey)
	go runConfirm(nonce2, photoKey2)
	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful check-in, got %d", successCount)
	}
	if duplicateCount != 1 {
		t.Errorf("expected exactly 1 ErrAlreadyCheckedIn, got %d", duplicateCount)
	}
}

// 7. Orphan photo cleanup on DB failure: discardPhoto removes the object from MinIO and rolls back.
func TestCheckIn_OrphanPhoto_CleanupOnDBFailure(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	userID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	// First successful check-in
	init1, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("first Initiate failed: %v", err)
	}
	_, err = svc.Confirm(ctx, userID, placeID, domain.CheckInConfirmRequest{
		CheckInNonce: init1.CheckInNonce,
		PhotoKey:     init1.PhotoKey,
	})
	if err != nil {
		t.Fatalf("first Confirm failed: %v", err)
	}

	// Second check-in on the same day: simulate that the client uploaded photo2,
	// but DB save will fail due to unique constraint idx_checkins_one_per_place_per_day.
	nonce2 := uuid.New().String()
	photoKey2 := fmt.Sprintf("checkins/%s/orphan.jpg", userID.String())
	mockStore.StoredFiles[photoKey2] = []byte("photo-bytes")

	session2 := service.CheckInSession{
		UserID:   userID,
		PlaceID:  placeID,
		PhotoKey: photoKey2,
		Lat:      11.0110,
		Lon:      -74.8080,
		IssuedAt: time.Now().UTC(),
	}
	enc, _ := json.Marshal(session2)
	_ = rdb.Set(ctx, service.CheckInNonceKey(nonce2), enc, 90*time.Second).Err()

	_, err = svc.Confirm(ctx, userID, placeID, domain.CheckInConfirmRequest{
		CheckInNonce: nonce2,
		PhotoKey:     photoKey2,
	})
	if !errors.Is(err, service.ErrAlreadyCheckedIn) {
		t.Fatalf("expected ErrAlreadyCheckedIn, got: %v", err)
	}

	// Verify that discardPhoto was called and photoKey2 was deleted from storage!
	if !mockStore.WasDeleted(photoKey2) {
		t.Errorf("expected photoKey2 to be deleted by discardPhoto upon DB failure, but it was not")
	}
}

// 8. Minor protection in check-ins: is_protected = true cannot initiate at minor_suitability = 'NO'.
func TestCheckIn_ProtectedMinor_RestrictedPlace(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	svc := service.NewCheckInService(cr, pr, mockStore, rdb)

	minorUser := testhelper.CreateTestUser(t, ctx, pool, true) // isProtected = true
	// Place 2: Bar El Rincón (minor_suitability = 'NO', lat: 11.0220, lon: -74.8200)
	restrictedPlace := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000002")

	_, err := svc.Initiate(ctx, minorUser, restrictedPlace, true, domain.CheckInInitiateRequest{
		Lat: 11.0220,
		Lon: -74.8200,
	})
	if !errors.Is(err, service.ErrPlaceRestricted) {
		t.Fatalf("expected ErrPlaceRestricted for protected minor at restricted place, got: %v", err)
	}
}
