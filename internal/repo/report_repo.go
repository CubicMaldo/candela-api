package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/nomad-api/internal/domain"
)

type ReportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{db: db}
}

// Create inserts a new report with status PENDING.
func (r *ReportRepository) Create(ctx context.Context, rep *domain.Report) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO reports (report_id, reporter_id, target_type, target_id, reason, comment, status, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, 'PENDING', NOW())
		RETURNING report_id, status, created_at
	`,
		rep.ReporterID, rep.TargetType, rep.TargetID, rep.Reason, rep.Comment,
	).Scan(&rep.ID, &rep.Status, &rep.CreatedAt)
}
