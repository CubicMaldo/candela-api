package domain

import (
	"time"

	"github.com/google/uuid"
)

type PointTransaction struct {
	TxID        uuid.UUID  `json:"tx_id"       db:"tx_id"`
	UserID      uuid.UUID  `json:"user_id"     db:"user_id"`
	Points      int        `json:"points"      db:"points"`
	Reason      string     `json:"reason"      db:"reason"`
	ReferenceID *uuid.UUID `json:"reference_id" db:"reference_id"`
	CreatedAt   time.Time  `json:"created_at"  db:"created_at"`
}

type Friendship struct {
	UserIDA   uuid.UUID `json:"user_id_a" db:"user_id_a"`
	UserIDB   uuid.UUID `json:"user_id_b" db:"user_id_b"`
	Status    string    `json:"status"    db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type FriendshipRequest struct {
	UserID uuid.UUID `json:"user_id" binding:"required"`
}
