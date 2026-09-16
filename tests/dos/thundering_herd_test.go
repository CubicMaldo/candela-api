package dos_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/tests/testhelper"
)

// spyRankingRepo records the number of times GetGlobal was executed against the database.
type spyRankingRepo struct {
	getGlobalCalls atomic.Int32
	getGlobalDelay time.Duration
	sampleEntries  []domain.RankingEntry
}

func (s *spyRankingRepo) GetGlobal(ctx context.Context, limit, offset int) ([]domain.RankingEntry, int, error) {
	s.getGlobalCalls.Add(1)
	if s.getGlobalDelay > 0 {
		time.Sleep(s.getGlobalDelay)
	}
	return s.sampleEntries, len(s.sampleEntries), nil
}

func (s *spyRankingRepo) GetUserGlobalRank(_ context.Context, _ uuid.UUID) (*domain.RankingEntry, error) {
	return &domain.RankingEntry{Rank: 1, PointsTotal: 500, BadgeLevel: "Callejero"}, nil
}

func (s *spyRankingRepo) GetFriends(_ context.Context, _ uuid.UUID) ([]domain.RankingEntry, error) {
	return s.sampleEntries, nil
}

func (s *spyRankingRepo) GetUserFriendRank(_ context.Context, _ uuid.UUID) (*domain.RankingEntry, error) {
	return &domain.RankingEntry{Rank: 1, PointsTotal: 500, BadgeLevel: "Callejero"}, nil
}

type fakeRankingUserRepo struct{}

func (f *fakeRankingUserRepo) GetByID(_ context.Context, userID uuid.UUID) (*domain.User, error) {
	return &domain.User{
		UserID:        userID,
		RankingOptIn:  true,
		AccountStatus: "ACTIVE",
		PrivacyMode:   "PUBLIC",
	}, nil
}

func (f *fakeRankingUserRepo) UpdateRankingOptIn(_ context.Context, _ uuid.UUID, _ bool) error {
	return nil
}

// TestThunderingHerd_SingleflightMitigation verifies that when 100 concurrent requests hit
// the ranking endpoint simultaneously on an expired/empty Redis cache, singleflight ensures
// the database query is invoked EXACTLY ONCE, preventing a database collapse (Cache Stampede).
func TestThunderingHerd_SingleflightMitigation(t *testing.T) {
	rdb := testhelper.GetTestRedis(t)
	if rdb == nil {
		return
	}
	ctx := context.Background()

	// Clear cache key first
	_ = rdb.Del(ctx, "rankings:global").Err()

	spy := &spyRankingRepo{
		getGlobalDelay: 30 * time.Millisecond, // Simulate non-trivial SQL calculation time
		sampleEntries: []domain.RankingEntry{
			{Rank: 1, UserID: uuid.New(), Username: "top1", BadgeLevel: "Pro", PointsTotal: 1000},
			{Rank: 2, UserID: uuid.New(), Username: "top2", BadgeLevel: "Pro", PointsTotal: 800},
		},
	}
	userRepo := &fakeRankingUserRepo{}

	svc := service.NewRankingServiceWithCustom(spy, userRepo, rdb)

	callerCount := 100
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})

	errorsCount := atomic.Int32{}
	successCount := atomic.Int32{}

	for i := 0; i < callerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier // Synchronize all goroutines to fire at the exact same instant

			testUser := uuid.New()
			resp, err := svc.GetGlobal(ctx, testUser)
			if err != nil || resp == nil || len(resp.Rankings) != 2 {
				errorsCount.Add(1)
			} else {
				successCount.Add(1)
			}
		}()
	}

	// Release all 100 requests in the exact same millisecond
	close(startBarrier)
	wg.Wait()

	if errorsCount.Load() > 0 {
		t.Fatalf("expected 0 errors across concurrent callers, got %d", errorsCount.Load())
	}
	if successCount.Load() != int32(callerCount) {
		t.Fatalf("expected %d successful callers, got %d", callerCount, successCount.Load())
	}

	// CRITICAL ASSERTION:
	// Without singleflight, getGlobalCalls would be ~100.
	// With singleflight, getGlobalCalls MUST be exactly 1!
	dbCalls := spy.getGlobalCalls.Load()
	if dbCalls != 1 {
		t.Fatalf("Thundering Herd vulnerability detected! Expected exactly 1 DB query for %d concurrent requests, got %d", callerCount, dbCalls)
	}

	// Verify Redis cache is now populated
	exists, err := rdb.Exists(ctx, "rankings:global").Result()
	if err != nil || exists == 0 {
		t.Errorf("expected Redis cache key 'rankings:global' to be set after singleflight completion")
	}

	// A subsequent call must read directly from Redis and NOT increment the DB calls
	_, err = svc.GetGlobal(ctx, uuid.New())
	if err != nil {
		t.Fatalf("subsequent call failed: %v", err)
	}
	if spy.getGlobalCalls.Load() != 1 {
		t.Errorf("subsequent call should have been served by Redis cache, DB calls: %d", spy.getGlobalCalls.Load())
	}
}

// TestThunderingHerd_EndToEnd_RealDB verifies 50 concurrent requests against live PostgreSQL and Redis.
func TestThunderingHerd_EndToEnd_RealDB(t *testing.T) {
	pool := testhelper.GetTestPool(t)
	rdb := testhelper.GetTestRedis(t)
	if pool == nil || rdb == nil {
		return
	}
	ctx := context.Background()

	// Clear cache key to simulate stampede
	_ = rdb.Del(ctx, "rankings:global").Err()

	rr := repo.NewRankingRepository(pool)
	ur := repo.NewUserRepository(pool)
	svc := service.NewRankingService(rr, ur, rdb)

	testUser := testhelper.CreateTestUser(t, ctx, pool, false)

	callerCount := 50
	var wg sync.WaitGroup
	startBarrier := make(chan struct{})
	successCount := atomic.Int32{}

	for i := 0; i < callerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier
			resp, err := svc.GetGlobal(ctx, testUser)
			if err == nil && resp != nil {
				successCount.Add(1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if successCount.Load() != int32(callerCount) {
		t.Fatalf("expected all %d concurrent requests to succeed, got %d", callerCount, successCount.Load())
	}
}
