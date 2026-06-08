package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

var (
	ErrCannotReportSelf     = errors.New("No puedes reportarte a ti mismo.")
	ErrReportTargetNotFound = errors.New("the reported content no longer exists")
)

type ReportService struct {
	reportRepo  *repo.ReportRepository
	userRepo    *repo.UserRepository
	placeRepo   *repo.PlaceRepository
	checkinRepo *repo.CheckInRepository
}

func NewReportService(rr *repo.ReportRepository, ur *repo.UserRepository, pr *repo.PlaceRepository, cr *repo.CheckInRepository) *ReportService {
	return &ReportService{reportRepo: rr, userRepo: ur, placeRepo: pr, checkinRepo: cr}
}

// CreateReport validates the target exists, rejects self-reports, and inserts
// a new report with status PENDING.
func (s *ReportService) CreateReport(ctx context.Context, reporterID uuid.UUID, req domain.CreateReportRequest) (*domain.Report, error) {
	if req.TargetType == domain.ReportTargetUser && req.TargetID == reporterID {
		return nil, ErrCannotReportSelf
	}

	exists, err := s.targetExists(ctx, req.TargetType, req.TargetID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrReportTargetNotFound
	}

	report := &domain.Report{
		ReporterID: reporterID,
		TargetType: req.TargetType,
		TargetID:   req.TargetID,
		Reason:     req.Reason,
		Comment:    req.Comment,
	}
	if err := s.reportRepo.Create(ctx, report); err != nil {
		return nil, err
	}
	return report, nil
}

func (s *ReportService) targetExists(ctx context.Context, targetType domain.ReportTargetType, targetID uuid.UUID) (bool, error) {
	switch targetType {
	case domain.ReportTargetUser:
		_, err := s.userRepo.FindByID(ctx, targetID)
		if err != nil {
			if errors.Is(err, repo.ErrUserNotFound) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	case domain.ReportTargetPlace:
		return s.placeRepo.IsActive(ctx, targetID)
	case domain.ReportTargetPhoto:
		return s.checkinRepo.Exists(ctx, targetID)
	default:
		return false, nil
	}
}
