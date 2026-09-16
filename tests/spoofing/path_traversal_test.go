package spoofing_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

func TestCheckIn_PathTraversalAndKeyInjection_Blocked(t *testing.T) {
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
	victimID := testhelper.CreateTestUser(t, ctx, pool, false)
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza

	traversalPayloads := []struct {
		name        string
		photoKey    string
		expectedErr error
	}{
		{
			name:        "classic dot-dot-slash traversal",
			photoKey:    "../../../../secrets/database.env",
			expectedErr: service.ErrInvalidPhotoKey,
		},
		{
			name:        "windows-style backslash traversal",
			photoKey:    fmt.Sprintf("checkins/%s/..\\..\\passwords.txt", userID.String()),
			expectedErr: service.ErrInvalidPhotoKey,
		},
		{
			name:        "url-encoded dot-dot traversal",
			photoKey:    fmt.Sprintf("checkins/%s/%%2e%%2e/config.json", userID.String()),
			expectedErr: service.ErrInvalidPhotoKey,
		},
		{
			name:        "victim user key injection (impersonation)",
			photoKey:    fmt.Sprintf("checkins/%s/2026-09-15/photo.jpg", victimID.String()),
			expectedErr: service.ErrInvalidPhotoKey,
		},
		{
			name:        "different photo key from the one issued in session",
			photoKey:    fmt.Sprintf("checkins/%s/2026-09-15/fake-photo.jpg", userID.String()),
			expectedErr: service.ErrInvalidCheckInNonce, // Mismatched key rejected by session validation
		},
	}

	for _, tt := range traversalPayloads {
		t.Run(tt.name, func(t *testing.T) {
			initResp, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
				Lat: 11.0110,
				Lon: -74.8080,
			})
			if err != nil {
				t.Fatalf("Initiate failed: %v", err)
			}

			_, cErr := svc.Confirm(ctx, userID, placeID, domain.CheckInConfirmRequest{
				CheckInNonce: initResp.CheckInNonce,
				PhotoKey:     tt.photoKey,
			})

			if !errors.Is(cErr, tt.expectedErr) {
				t.Fatalf("expected error %v for key %q, got: %v", tt.expectedErr, tt.photoKey, cErr)
			}
		})
	}
}
