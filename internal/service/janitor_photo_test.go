package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/testhelper"
)

// 5.1 Regla de foto permanente (SetPermanent)
// - Probar que un usuario no puede modificar el estado permanente del check-in de otro usuario (ErrNotCheckInOwner).
// - Probar que un usuario solo puede marcar su foto como permanente mientras haya actividad reciente en el lugar (últimas 4h). Si el evento cerró, debe retornar ErrEventClosed.
// - Probar que si el evento sigue activo, la foto se marca exitosamente como permanente.
func TestCheckIn_SetPermanent(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	pr := repo.NewPlanRepository(pool)
	checkInSvc := NewCheckInService(cr, pr, mockStore, rdb)

	userOwner := createTestUser(t, ctx, pool, false)
	userOther := createTestUser(t, ctx, pool, false)

	// Create an isolated test place with no other check-ins
	testPlaceID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO places (place_id, name, description, category, minor_suitability, location, geofence_meters, is_active)
		VALUES ($1, 'Lugar Temporal Test', 'Desc', 'BAR', 'YES', ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, 100, true)
	`, testPlaceID)
	if err != nil {
		t.Fatalf("failed to insert test place: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM check_ins WHERE place_id = $1`, testPlaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM places WHERE place_id = $1`, testPlaceID)
	})

	// 1. Insert a check-in that happened 26 hours ago (event has closed, and different calendar day)
	oldCheckInID := uuid.New()
	photoKey := fmt.Sprintf("checkins/%s/old.jpg", userOwner.String())
	photoURL := fmt.Sprintf("http://mock-storage/%s", photoKey)
	expiresAt := time.Now().Add(19 * time.Hour)

	_, err = pool.Exec(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_key, photo_expires_at, photo_is_permanent,
			location, gps_valid, timestamp_valid, validation_passed,
			points_awarded, is_first_of_day, checked_in_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, false,
			ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
			10, true, NOW() - INTERVAL '26 hours'
		)
	`, oldCheckInID, userOwner, testPlaceID, photoURL, photoKey, expiresAt)
	if err != nil {
		t.Fatalf("failed to insert old checkin: %v", err)
	}

	// 1.1 Modifying another user's check-in fails with ErrNotCheckInOwner
	_, err = checkInSvc.SetPermanent(ctx, userOther, oldCheckInID, true)
	if !errors.Is(err, ErrNotCheckInOwner) {
		t.Fatalf("expected ErrNotCheckInOwner for non-owner, got: %v", err)
	}

	// 1.2 Modifying check-in when event is closed (0 check-ins in last 4 hours) fails with ErrEventClosed
	_, err = checkInSvc.SetPermanent(ctx, userOwner, oldCheckInID, true)
	if !errors.Is(err, ErrEventClosed) {
		t.Fatalf("expected ErrEventClosed when event is closed, got: %v", err)
	}

	// 2. Now add an active check-in at this place in the last 4 hours (event is now active!)
	activeCheckInID := uuid.New()
	activeKey := fmt.Sprintf("checkins/%s/active.jpg", userOwner.String())
	activeURL := fmt.Sprintf("http://mock-storage/%s", activeKey)

	_, err = pool.Exec(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_key, photo_expires_at, photo_is_permanent,
			location, gps_valid, timestamp_valid, validation_passed,
			points_awarded, is_first_of_day, checked_in_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6, false,
			ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
			10, false, NOW() - INTERVAL '30 minutes'
		)
	`, activeCheckInID, userOwner, testPlaceID, activeURL, activeKey, expiresAt)
	if err != nil {
		t.Fatalf("failed to insert active checkin: %v", err)
	}

	// 2.1 Owner marking photo as permanent while event is active succeeds!
	updated, err := checkInSvc.SetPermanent(ctx, userOwner, activeCheckInID, true)
	if err != nil {
		t.Fatalf("SetPermanent failed for active event: %v", err)
	}
	if !updated.PhotoIsPermanent {
		t.Errorf("expected PhotoIsPermanent to be true")
	}

	// Verify in DB that photo_is_permanent is true
	var isPermanent bool
	err = pool.QueryRow(ctx, `SELECT photo_is_permanent FROM check_ins WHERE checkin_id = $1`, activeCheckInID).Scan(&isPermanent)
	if err != nil || !isPermanent {
		t.Errorf("expected DB photo_is_permanent to be true, got %v, err: %v", isPermanent, err)
	}
}

// 5.2 Worker de limpieza en segundo plano (Janitor)
// - Probar que el Janitor identifica fotos con photo_expires_at <= NOW() que no sean permanentes.
// - Borra el archivo de MinIO (storage.Delete).
// - Limpia la referencia en la base de datos (photo_key = NULL, photo_url = NULL).
// - No toca fotos permanentes ni fotos que aún no han expirado.
func TestJanitor_SweepExpiredPhotos(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	mockStore := testhelper.NewMockStorage()
	cr := repo.NewCheckInRepository(pool, "America/Bogota")
	rr := repo.NewRefreshTokenRepository(pool)
	janitor := NewJanitor(cr, rr, mockStore)

	userExpired := createTestUser(t, ctx, pool, false)
	userPerm := createTestUser(t, ctx, pool, false)
	userFuture := createTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	// Photo 1: Expired Ephemeral (SHOULD BE CLEANED)
	idExpired := uuid.New()
	keyExpired := fmt.Sprintf("checkins/%s/exp_%s.jpg", userExpired.String(), idExpired.String()[:8])
	mockStore.StoredFiles[keyExpired] = []byte("expired image bytes")

	_, err := pool.Exec(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_key, photo_expires_at, photo_is_permanent,
			location, gps_valid, timestamp_valid, validation_passed,
			points_awarded, is_first_of_day, checked_in_at
		) VALUES (
			$1, $2, $3,
			'http://mock/' || $4, $4, NOW() - INTERVAL '2 hours', false,
			ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
			10, false, NOW() - INTERVAL '26 hours'
		)
	`, idExpired, userExpired, placeID, keyExpired)
	if err != nil {
		t.Fatalf("failed to insert expired checkin: %v", err)
	}

	// Photo 2: Expired but PERMANENT (SHOULD NOT BE CLEANED)
	idPermanent := uuid.New()
	keyPermanent := fmt.Sprintf("checkins/%s/perm_%s.jpg", userPerm.String(), idPermanent.String()[:8])
	mockStore.StoredFiles[keyPermanent] = []byte("permanent image bytes")

	_, err = pool.Exec(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_key, photo_expires_at, photo_is_permanent,
			location, gps_valid, timestamp_valid, validation_passed,
			points_awarded, is_first_of_day, checked_in_at
		) VALUES (
			$1, $2, $3,
			'http://mock/' || $4, $4, NOW() - INTERVAL '2 hours', true,
			ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
			10, false, NOW() - INTERVAL '26 hours'
		)
	`, idPermanent, userPerm, placeID, keyPermanent)
	if err != nil {
		t.Fatalf("failed to insert permanent checkin: %v", err)
	}

	// Photo 3: Ephemeral but NOT YET EXPIRED (SHOULD NOT BE CLEANED)
	idFuture := uuid.New()
	keyFuture := fmt.Sprintf("checkins/%s/future_%s.jpg", userFuture.String(), idFuture.String()[:8])
	mockStore.StoredFiles[keyFuture] = []byte("future image bytes")

	_, err = pool.Exec(ctx, `
		INSERT INTO check_ins (
			checkin_id, user_id, place_id,
			photo_url, photo_key, photo_expires_at, photo_is_permanent,
			location, gps_valid, timestamp_valid, validation_passed,
			points_awarded, is_first_of_day, checked_in_at
		) VALUES (
			$1, $2, $3,
			'http://mock/' || $4, $4, NOW() + INTERVAL '20 hours', false,
			ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
			10, false, NOW() - INTERVAL '4 hours'
		)
	`, idFuture, userFuture, placeID, keyFuture)
	if err != nil {
		t.Fatalf("failed to insert future checkin: %v", err)
	}

	// Run Janitor sweep
	janitor.sweepExpiredPhotos(ctx)

	// 1. Verify expired ephemeral photo was deleted from storage
	if !mockStore.WasDeleted(keyExpired) {
		t.Errorf("expected expired photo %s to be deleted from storage", keyExpired)
	}

	// 2. Verify expired ephemeral photo was cleared in the DB (photo_key and photo_url are NULL)
	var dbKey, dbURL *string
	err = pool.QueryRow(ctx, `SELECT photo_key, photo_url FROM check_ins WHERE checkin_id = $1`, idExpired).Scan(&dbKey, &dbURL)
	if err != nil {
		t.Fatalf("querying expired checkin failed: %v", err)
	}
	if dbKey != nil || dbURL != nil {
		t.Errorf("expected photo_key and photo_url to be NULL in DB for expired photo, got key=%v, url=%v", dbKey, dbURL)
	}

	// 3. Verify permanent photo was NOT deleted from storage
	if mockStore.WasDeleted(keyPermanent) {
		t.Errorf("permanent photo %s should NOT have been deleted from storage", keyPermanent)
	}
	// Verify permanent photo was NOT cleared in DB
	err = pool.QueryRow(ctx, `SELECT photo_key, photo_url FROM check_ins WHERE checkin_id = $1`, idPermanent).Scan(&dbKey, &dbURL)
	if err != nil || dbKey == nil || *dbKey != keyPermanent {
		t.Errorf("expected permanent photo_key to remain %s, got %v", keyPermanent, dbKey)
	}

	// 4. Verify future photo was NOT deleted from storage
	if mockStore.WasDeleted(keyFuture) {
		t.Errorf("future photo %s should NOT have been deleted from storage", keyFuture)
	}
	// Verify future photo was NOT cleared in DB
	err = pool.QueryRow(ctx, `SELECT photo_key, photo_url FROM check_ins WHERE checkin_id = $1`, idFuture).Scan(&dbKey, &dbURL)
	if err != nil || dbKey == nil || *dbKey != keyFuture {
		t.Errorf("expected future photo_key to remain %s, got %v", keyFuture, dbKey)
	}
}
