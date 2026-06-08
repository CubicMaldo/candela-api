package domain

import (
	"time"

	"github.com/google/uuid"
)

type Follow struct {
	FollowerID uuid.UUID `json:"follower_id" db:"follower_id"`
	PlaceID    uuid.UUID `json:"place_id"    db:"place_id"`
	CreatedAt  time.Time `json:"created_at"  db:"created_at"`
}
