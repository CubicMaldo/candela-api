package service

import (
	"context"
	"log"
	"time"

	"github.com/tuusuario/candela-api/internal/infra/storage"
	"github.com/tuusuario/candela-api/internal/repo"
)

const (
	janitorInterval  = time.Hour
	janitorBatchSize = 200
)

// Janitor reclaims storage and database rows that nothing references any more.
//
// Ephemeral photos previously leaked forever: photo_expires_at only hid the URL
// from queries, while the object itself stayed in the bucket and kept costing
// money. Expired refresh tokens accumulated the same way.
//
// Safe to run on every API instance: each sweep claims its work with
// FOR UPDATE SKIP LOCKED, so instances take disjoint batches.
type Janitor struct {
	checkinRepo *repo.CheckInRepository
	refreshRepo *repo.RefreshTokenRepository
	storage     storage.ObjectStorage
}

func NewJanitor(cr *repo.CheckInRepository, rr *repo.RefreshTokenRepository, st storage.ObjectStorage) *Janitor {
	return &Janitor{checkinRepo: cr, refreshRepo: rr, storage: st}
}

// Run sweeps once at startup and then on every tick, until ctx is cancelled.
func (j *Janitor) Run(ctx context.Context) {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()

	j.sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.sweep(ctx)
		}
	}
}

func (j *Janitor) sweep(ctx context.Context) {
	j.sweepExpiredPhotos(ctx)

	removed, err := j.refreshRepo.DeleteExpired(ctx)
	if err != nil {
		log.Printf("janitor: refresh token cleanup failed: %v", err)
	} else if removed > 0 {
		log.Printf("janitor: removed %d expired refresh tokens", removed)
	}
}

// sweepExpiredPhotos deletes ephemeral photos past their viewing window,
// in batches so one sweep never holds a large transaction open.
func (j *Janitor) sweepExpiredPhotos(ctx context.Context) {
	total := 0
	for {
		if ctx.Err() != nil {
			return
		}

		keys, err := j.checkinRepo.ClaimExpiredPhotos(ctx, janitorBatchSize)
		if err != nil {
			log.Printf("janitor: could not claim expired photos: %v", err)
			return
		}
		if len(keys) == 0 {
			break
		}

		for _, key := range keys {
			if err := j.storage.Delete(ctx, key); err != nil {
				// The row was already cleared, so this object is now
				// unreferenced. Log it: it needs a bucket lifecycle rule or a
				// manual pass, and retrying here would spin forever.
				log.Printf("janitor: could not delete expired photo %s: %v", key, err)
			}
		}
		total += len(keys)

		// A short batch means the queue is drained.
		if len(keys) < janitorBatchSize {
			break
		}
	}

	if total > 0 {
		log.Printf("janitor: swept %d expired photos", total)
	}
}
