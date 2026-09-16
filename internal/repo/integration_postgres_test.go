package repo

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tuusuario/candela-api/internal/testhelper"
)

// 1. PostGIS Geofencing:
// ST_DWithin y cálculos geoespaciales reales ejecutándose contra PostgreSQL + PostGIS.
func TestPostGIS_Geofencing(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	cr := NewCheckInRepository(pool, "America/Bogota")

	testPlaceID := uuid.New()
	centerLon := -74.8080
	centerLat := 11.0110
	geofenceMeters := 80.0

	_, err := pool.Exec(ctx, `
		INSERT INTO places (place_id, name, description, category, minor_suitability, location, geofence_meters, is_active)
		VALUES ($1, 'Geofence Test Place', 'Test', 'CAFE', 'YES',
		        ST_SetSRID(ST_MakePoint($2, $3), 4326)::geography, $4, true)
	`, testPlaceID, centerLon, centerLat, geofenceMeters)
	if err != nil {
		t.Fatalf("failed to insert test place: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM check_ins WHERE place_id = $1`, testPlaceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM places WHERE place_id = $1`, testPlaceID)
	})

	// 1. Exact center (distance = 0m) -> must be within
	within, err := cr.IsWithinGeofence(ctx, testPlaceID, centerLat, centerLon)
	if err != nil || !within {
		t.Errorf("center point expected to be within geofence, got within=%v, err=%v", within, err)
	}

	// 2. Point ~30m away (lat + 0.00025 deg in Barranquilla is ~27.7 meters) -> must be within
	latClose := centerLat + 0.00025
	lonClose := centerLon
	within, err = cr.IsWithinGeofence(ctx, testPlaceID, latClose, lonClose)
	if err != nil || !within {
		t.Errorf("point ~30m away expected to be within geofence, got within=%v, err=%v", within, err)
	}

	// 3. Point ~150m away (lat + 0.0014 deg in Barranquilla is ~155 meters) -> must be outside (geofence is 80m)
	latFar := centerLat + 0.0014
	lonFar := centerLon
	within, err = cr.IsWithinGeofence(ctx, testPlaceID, latFar, lonFar)
	if err != nil {
		t.Errorf("unexpected error checking far point: %v", err)
	}
	if within {
		t.Errorf("point ~150m away expected to be OUTSIDE 80m geofence, but got within=true")
	}

	// 4. Coordinates swapped test:
	// If longitude and latitude were accidentally swapped, distance would be thousands of km
	within, err = cr.IsWithinGeofence(ctx, testPlaceID, centerLon, centerLat)
	if within {
		t.Errorf("swapped lat/lon should not be within geofence")
	}

	// 5. Inactive place: geofence must reject if place is not active
	_, err = pool.Exec(ctx, `UPDATE places SET is_active = false WHERE place_id = $1`, testPlaceID)
	if err != nil {
		t.Fatalf("failed to deactivate place: %v", err)
	}
	within, err = cr.IsWithinGeofence(ctx, testPlaceID, centerLat, centerLon)
	// Query filters WHERE is_active = true, so no rows found -> within is false
	if within {
		t.Errorf("inactive place should NOT be within geofence")
	}
}

// 2. Índice único con Zona Horaria:
// idx_checkins_one_per_place_per_day utiliza ((checked_in_at AT TIME ZONE 'America/Bogota')::date).
// Dos check-ins a las 11:59 PM y 12:01 AM de Bogotá se comportan como días distintos,
// aun cuando en UTC ambos ocurren en la misma fecha calendario.
func TestCheckIn_UniqueIndex_Timezone_AmericaBogota(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	userID := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO users (user_id, email, username, password_hash, birth_date, is_protected, account_status, privacy_mode)
		VALUES ($1, $2, $3, 'hash', '2000-01-01', false, 'ACTIVE', 'PUBLIC')
	`, userID, fmt.Sprintf("tz-%s@test.com", userID.String()[:8]), fmt.Sprintf("tz_%s", userID.String()[:8]))
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM point_transactions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM check_ins WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE user_id = $1`, userID)
	})

	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza

	// Escenario temporal:
	// Check-in 1: 2026-10-15 23:59:00 Bogotá (-05:00) -> 2026-10-16 04:59:00 UTC
	// Check-in 2: 2026-10-16 00:01:00 Bogotá (-05:00) -> 2026-10-16 05:01:00 UTC
	//
	// ¡En UTC, ambos check-ins ocurrieron el 2026-10-16!
	// Pero en America/Bogota:
	//   Check-in 1 es 2026-10-15
	//   Check-in 2 es 2026-10-16
	// El índice idx_checkins_one_per_place_per_day DEBE permitir ambos.

	ts1Bogota := "2026-10-16 04:59:00+00" // 11:59 PM en Bogotá el día 15
	ts2Bogota := "2026-10-16 05:01:00+00" // 12:01 AM en Bogotá el día 16 (solo 2 minutos después)
	ts3Bogota := "2026-10-16 05:30:00+00" // 12:30 AM en Bogotá el día 16 (mismo día que Check-in 2)

	ci1 := uuid.New()
	ci2 := uuid.New()
	ci3 := uuid.New()

	insertCheckInAt := func(id uuid.UUID, timestamptzStr string) error {
		_, execErr := pool.Exec(ctx, `
			INSERT INTO check_ins (
				checkin_id, user_id, place_id,
				location, gps_valid, timestamp_valid, validation_passed,
				points_awarded, is_first_of_day, checked_in_at
			) VALUES (
				$1, $2, $3,
				ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, true,
				10, true, $4::timestamptz
			)
		`, id, userID, placeID, timestamptzStr)
		return execErr
	}

	// 1. Insert Check-in 1 (11:59 PM Bogotá) -> DEBE TENER ÉXITO
	if err := insertCheckInAt(ci1, ts1Bogota); err != nil {
		t.Fatalf("Check-in 1 at 11:59 PM failed: %v", err)
	}

	// 2. Insert Check-in 2 (12:01 AM Bogotá, 2 minutos después) -> DEBE TENER ÉXITO
	// Porque aunque en UTC es la misma fecha (2026-10-16), en Bogotá cruzó la medianoche a un nuevo día calendario.
	if err := insertCheckInAt(ci2, ts2Bogota); err != nil {
		t.Fatalf("Check-in 2 at 12:01 AM next day in Bogota should succeed, but failed: %v", err)
	}

	// 3. Insert Check-in 3 (12:30 AM Bogotá) -> DEBE FALLAR con violación de unicidad
	// Porque está en la misma fecha local en Bogotá que Check-in 2 (2026-10-16).
	err = insertCheckInAt(ci3, ts3Bogota)
	if err == nil {
		t.Fatalf("expected Check-in 3 on same Bogota day to fail with unique violation, but succeeded")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "idx_checkins_one_per_place_per_day" {
		t.Fatalf("expected unique constraint violation idx_checkins_one_per_place_per_day (23505), got: %v", err)
	}
}

// 3. Integridad referencial y cascadas en DeleteAccount:
// Verificar que el borrado elimina amistades, participaciones en planes, follows,
// check-ins, transacciones de puntos y tokens sin romper restricciones de clave foránea,
// y anonimiza la fila en users.
func TestDeleteAccount_ReferentialIntegrityAndCascades(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	userRepo := NewUserRepository(pool)
	refreshRepo := NewRefreshTokenRepository(pool)

	// Create user to delete and peer user
	userToDelete := uuid.New()
	userPeer := uuid.New()

	for _, u := range []struct {
		id    uuid.UUID
		email string
		name  string
	}{
		{userToDelete, "victim@candela.test", "user_victim"},
		{userPeer, "peer@candela.test", "user_peer"},
	} {
		_, err := pool.Exec(ctx, `
			INSERT INTO users (user_id, email, username, password_hash, birth_date, is_protected, account_status, privacy_mode, points_total)
			VALUES ($1, $2, $3, 'hash', '2000-01-01', false, 'ACTIVE', 'PUBLIC', 300)
		`, u.id, u.email, u.name)
		if err != nil {
			t.Fatalf("failed to insert user %s: %v", u.name, err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE user_id IN ($1, $2)`, userToDelete, userPeer)
	})

	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001")

	// 1. Create Friendship
	idA, idB := userToDelete, userPeer
	if idA.String() > idB.String() {
		idA, idB = idB, idA
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO friendships (user_id_a, user_id_b, requester_id, status, created_at)
		VALUES ($1, $2, $3, 'accepted', NOW())
	`, idA, idB, userToDelete)
	if err != nil {
		t.Fatalf("failed to insert friendship: %v", err)
	}

	// 2. Create Plan by userToDelete where userPeer participates
	plan1ID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO plans (plan_id, creator_user_id, target_user_id, place_id, status, visibility, deadline_hours, deadline_at)
		VALUES ($1, $2, $3, $4, 'ACCEPTED', 'INVITE', 24, NOW() + INTERVAL '20 hours')
	`, plan1ID, userToDelete, userPeer, placeID)
	if err != nil {
		t.Fatalf("failed to insert plan 1: %v", err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO plan_participants (plan_id, user_id, role, status) VALUES ($1, $2, 'CREATOR', 'JOINED')`, plan1ID, userToDelete)
	_, _ = pool.Exec(ctx, `INSERT INTO plan_participants (plan_id, user_id, role, status) VALUES ($1, $2, 'GUEST', 'JOINED')`, plan1ID, userPeer)

	// 3. Create Plan by userPeer where userToDelete participates
	plan2ID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO plans (plan_id, creator_user_id, target_user_id, place_id, status, visibility, deadline_hours, deadline_at)
		VALUES ($1, $2, $3, $4, 'ACCEPTED', 'INVITE', 24, NOW() + INTERVAL '20 hours')
	`, plan2ID, userPeer, userToDelete, placeID)
	if err != nil {
		t.Fatalf("failed to insert plan 2: %v", err)
	}
	_, _ = pool.Exec(ctx, `INSERT INTO plan_participants (plan_id, user_id, role, status) VALUES ($1, $2, 'CREATOR', 'JOINED')`, plan2ID, userPeer)
	_, _ = pool.Exec(ctx, `INSERT INTO plan_participants (plan_id, user_id, role, status) VALUES ($1, $2, 'GUEST', 'JOINED')`, plan2ID, userToDelete)

	// 4. Follow place
	_, err = pool.Exec(ctx, `INSERT INTO follows (follower_id, place_id) VALUES ($1, $2)`, userToDelete, placeID)
	if err != nil {
		t.Fatalf("failed to insert follow: %v", err)
	}

	// 5. Check-in with photo
	ciID := uuid.New()
	photoKey := fmt.Sprintf("checkins/%s/del.jpg", userToDelete.String())
	_, err = pool.Exec(ctx, `
		INSERT INTO check_ins (checkin_id, user_id, place_id, photo_url, photo_key, location, gps_valid, validation_passed, points_awarded, is_first_of_day, checked_in_at)
		VALUES ($1, $2, $3, 'http://url', $4, ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, true, true, 10, true, NOW())
	`, ciID, userToDelete, placeID, photoKey)
	if err != nil {
		t.Fatalf("failed to insert checkin: %v", err)
	}

	// 6. Point transaction
	_, err = pool.Exec(ctx, `
		INSERT INTO point_transactions (tx_id, user_id, points, reason, reference_id)
		VALUES (uuid_generate_v4(), $1, 10, 'CHECKIN', $2)
	`, userToDelete, ciID)
	if err != nil {
		t.Fatalf("failed to insert point transaction: %v", err)
	}

	// 7. Refresh token
	err = refreshRepo.Create(ctx, userToDelete, "rt-token-delete-test", time.Now().Add(24*time.Hour))
	if err != nil {
		t.Fatalf("failed to insert refresh token: %v", err)
	}

	// EXECUTE DeleteAccount: MUST NOT FAIL WITH FOREIGN KEY ERROR
	photoKeys, err := userRepo.DeleteAccount(ctx, userToDelete)
	if err != nil {
		t.Fatalf("DeleteAccount failed with error (possible foreign key violation): %v", err)
	}

	// Also revoke refresh tokens as UserService does
	_ = refreshRepo.RevokeAllForUser(ctx, userToDelete)

	// Verify photoKeys contains the check-in photo
	foundKey := false
	for _, k := range photoKeys {
		if k == photoKey {
			foundKey = true
			break
		}
	}
	if !foundKey {
		t.Errorf("expected photoKey %s in returned photoKeys, got: %v", photoKey, photoKeys)
	}

	// Verify cascade deletions in tables:
	var count int

	// Friendships
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM friendships WHERE user_id_a = $1 OR user_id_b = $1`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 friendships for deleted user, got %d", count)
	}

	// Plans created by or targeting userToDelete
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM plans WHERE creator_user_id = $1 OR target_user_id = $1`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 plans for deleted user, got %d", count)
	}

	// Plan participants
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM plan_participants WHERE user_id = $1`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 plan_participants for deleted user, got %d", count)
	}

	// Follows
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM follows WHERE follower_id = $1`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 follows for deleted user, got %d", count)
	}

	// Check-ins
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM check_ins WHERE user_id = $1`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 check_ins for deleted user, got %d", count)
	}

	// Point transactions
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM point_transactions WHERE user_id = $1`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 point_transactions for deleted user, got %d", count)
	}

	// Refresh tokens (revoked)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`, userToDelete).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 active refresh_tokens for deleted user, got %d", count)
	}

	// Verify user row is anonymized:
	var status, email, username, privacyMode string
	var pointsTotal int
	err = pool.QueryRow(ctx, `
		SELECT account_status, email, username, privacy_mode, points_total
		FROM users WHERE user_id = $1
	`, userToDelete).Scan(&status, &email, &username, &privacyMode, &pointsTotal)
	if err != nil {
		t.Fatalf("querying deleted user failed: %v", err)
	}

	if status != "DELETED" {
		t.Errorf("expected account_status DELETED, got %s", status)
	}
	expectedEmail := fmt.Sprintf("deleted_%s@candela.deleted", userToDelete.String())
	if email != expectedEmail {
		t.Errorf("expected email %s, got %s", expectedEmail, email)
	}
	expectedUsername := fmt.Sprintf("deleted_%s", userToDelete.String())
	if username != expectedUsername {
		t.Errorf("expected username %s, got %s", expectedUsername, username)
	}
	if privacyMode != "PRIVATE" {
		t.Errorf("expected privacy_mode PRIVATE, got %s", privacyMode)
	}
	if pointsTotal != 0 {
		t.Errorf("expected points_total 0, got %d", pointsTotal)
	}
}

// 4. PostgreSQL Constraint Geofence:
// Verifica que la tabla places rechace geofence_meters < 20 o > 150 a nivel de base de datos.
func TestPostGIS_Geofence_Constraint_20_to_150(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	if pool == nil {
		return
	}
	ctx := context.Background()

	insertPlaceWithGeofence := func(meters int) error {
		id := uuid.New()
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM places WHERE place_id = $1`, id)
		})
		_, err := pool.Exec(ctx, `
			INSERT INTO places (place_id, name, description, category, minor_suitability, location, geofence_meters, is_active)
			VALUES ($1, 'Constraint Test', 'Desc', 'CAFE', 'YES',
			        ST_SetSRID(ST_MakePoint(-74.8080, 11.0110), 4326)::geography, $2, true)
		`, id, meters)
		return err
	}

	// 1. Below minimum (15m) -> must fail check constraint
	err := insertPlaceWithGeofence(15)
	if err == nil {
		t.Fatalf("expected error for geofence_meters 15, but succeeded")
	}

	// 2. Exact minimum (20m) -> must succeed
	if err := insertPlaceWithGeofence(20); err != nil {
		t.Fatalf("expected geofence_meters 20 to succeed, got: %v", err)
	}

	// 3. Exact maximum (150m) -> must succeed
	if err := insertPlaceWithGeofence(150); err != nil {
		t.Fatalf("expected geofence_meters 150 to succeed, got: %v", err)
	}

	// 4. Above maximum (151m) -> must fail check constraint
	err = insertPlaceWithGeofence(151)
	if err == nil {
		t.Fatalf("expected error for geofence_meters 151, but succeeded")
	}
}

