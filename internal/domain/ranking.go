package domain

import "github.com/google/uuid"

type RankingEntry struct {
	Rank        int       `json:"rank"`
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	BadgeLevel  string    `json:"badge_level"`
	PointsTotal int       `json:"points_total"`
}

type RankingsResponse struct {
	Rankings []RankingEntry `json:"rankings"`
	Total    int            `json:"total"`
	UserRank *RankingEntry  `json:"user_rank"`
}
