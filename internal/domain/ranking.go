package domain

import "github.com/google/uuid"

type RankingEntry struct {
	Rank        int       `json:"rank"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	BadgeLevel  string    `json:"badge_level"`
	PointsTotal int       `json:"points_total"`
}

// RankingOptInRequest is sent to enable or disable participation in the global ranking.
// A pointer is used so the binder can distinguish "absent" from "false".
type RankingOptInRequest struct {
	RankingOptIn *bool `json:"ranking_opt_in" binding:"required"`
}

type RankingsResponse struct {
	Rankings       []RankingEntry `json:"rankings"`
	Total          int            `json:"total"`
	UserRank       *RankingEntry  `json:"user_rank"`
	UserRankReason *string        `json:"user_rank_reason,omitempty"`
}
