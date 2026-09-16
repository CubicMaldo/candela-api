package race_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

// TestCheckIn_Confirm_DoubleSpending_50ConcurrentRequests tests that when an attacker
// sends 50 parallel requests in the exact same millisecond with the exact same nonce,
// atomic consumption (GetDel) guarantees:
// 1. EXACTLY 1 request succeeds.
// 2. EXACTLY 49 requests are rejected with ErrInvalidCheckInNonce at the Redis layer.
// 3. PostgreSQL contains exactly 1 check-in record and 1 point transaction (NO double points).
// 4. The winner's photo is NOT deleted by any of the 49 losing requests.
func TestCheckIn_Confirm_DoubleSpending_50ConcurrentRequests(t *testing.T) {
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
	placeID := uuid.MustParse("bbbbbbbb-0000-0000-0000-000000000001") // Cafetería La Plaza

	// 1. Legitimate Initiate gives a single valid nonce
	initResp, err := svc.Initiate(ctx, userID, placeID, false, domain.CheckInInitiateRequest{
		Lat: 11.0110,
		Lon: -74.8080,
	})
	if err != nil {
		t.Fatalf("Initiate failed: %v", err)
	}

	// Pre-populate mock storage with the uploaded photo
	mockStore.StoredFiles[initResp.PhotoKey] = []byte("valid-photo-data")

	confirmReq := domain.CheckInConfirmRequest{
		CheckInNonce: initResp.CheckInNonce,
		PhotoKey:     initResp.PhotoKey,
	}

	concurrency := 50
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	successCount := atomic.Int32{}
	nonceErrorCount := atomic.Int32{}
	otherErrorCount := atomic.Int32{}

	// 2. Launch 50 goroutines ready to fire concurrently
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier // Synchronize all goroutines to fire at the exact same millisecond

			res, cErr := svc.Confirm(ctx, userID, placeID, confirmReq)
			if cErr == nil && res != nil && res.CheckIn.CheckInID != uuid.Nil {
				successCount.Add(1)
			} else if errors.Is(cErr, service.ErrInvalidCheckInNonce) {
				nonceErrorCount.Add(1)
			} else {
				otherErrorCount.Add(1)
			}
		}()
	}

	// Release all 50 goroutines simultaneously
	close(startBarrier)
	wg.Wait()

	// 3. Verify exactly 1 success and 49 ErrInvalidCheckInNonce
	if successCount.Load() != 1 {
		t.Fatalf("RACE CONDITION: Expected exactly 1 successful confirm, got %d", successCount.Load())
	}
	if nonceErrorCount.Load() != int32(concurrency-1) {
		t.Fatalf("Expected %d ErrInvalidCheckInNonce rejections, got %d (other errors: %d)",
			concurrency-1, nonceErrorCount.Load(), otherErrorCount.Load())
	}

	// 4. Verify database has exactly 1 check-in row for this user today
	var checkinCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM check_ins WHERE user_id = $1 AND place_id = $2`, userID, placeID).Scan(&checkinCount)
	if err != nil {
		t.Fatalf("querying check_ins failed: %v", err)
	}
	if checkinCount != 1 {
		t.Errorf("expected exactly 1 check_in row in DB, got %d", checkinCount)
	}

	// 5. Verify database has exactly 1 point transaction (NO double spending / point duplication)
	var pointTxCount int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM point_transactions WHERE user_id = $1`, userID).Scan(&pointTxCount)
	if err != nil {
		t.Fatalf("querying point_transactions failed: %v", err)
	}
	if pointTxCount != 1 {
		t.Errorf("expected exactly 1 point transaction, got %d", pointTxCount)
	}

	// 6. CRITICAL: Verify the winner's photo was NOT deleted from storage by losing requests
	if mockStore.WasDeleted(initResp.PhotoKey) {
		t.Errorf("CRITICAL BUG: Winner's photo %s was mistakenly deleted from storage by concurrent requests!", initResp.PhotoKey)
	}
}
