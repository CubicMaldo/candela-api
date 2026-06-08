package domain

import (
	"time"

	"github.com/google/uuid"
)

type ReportTargetType string
type ReportReason string
type ReportStatus string

const (
	ReportTargetPhoto ReportTargetType = "PHOTO"
	ReportTargetUser  ReportTargetType = "USER"
	ReportTargetPlace ReportTargetType = "PLACE"

	ReportReasonInappropriate ReportReason = "INAPPROPRIATE_CONTENT"
	ReportReasonHarassment    ReportReason = "HARASSMENT"
	ReportReasonSpam          ReportReason = "SPAM"
	ReportReasonFake          ReportReason = "FAKE_CONTENT"
	ReportReasonOther         ReportReason = "OTHER"

	ReportStatusPending   ReportStatus = "PENDING"
	ReportStatusReviewed  ReportStatus = "REVIEWED"
	ReportStatusDismissed ReportStatus = "DISMISSED"
)

// CreateReportRequest is sent by the client to report content, a user, or a place.
type CreateReportRequest struct {
	TargetType ReportTargetType `json:"target_type" binding:"required,oneof=PHOTO USER PLACE"`
	TargetID   uuid.UUID        `json:"target_id"   binding:"required"`
	Reason     ReportReason     `json:"reason"      binding:"required,oneof=INAPPROPRIATE_CONTENT HARASSMENT SPAM FAKE_CONTENT OTHER"`
	Comment    *string          `json:"comment,omitempty"`
}

type Report struct {
	ID         uuid.UUID        `json:"id"          db:"report_id"`
	ReporterID uuid.UUID        `json:"reporter_id" db:"reporter_id"`
	TargetType ReportTargetType `json:"target_type" db:"target_type"`
	TargetID   uuid.UUID        `json:"target_id"   db:"target_id"`
	Reason     ReportReason     `json:"reason"      db:"reason"`
	Comment    *string          `json:"comment,omitempty" db:"comment"`
	Status     ReportStatus     `json:"status"      db:"status"`
	CreatedAt  time.Time        `json:"created_at"  db:"created_at"`
}
