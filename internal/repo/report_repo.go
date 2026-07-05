package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
)

var ErrDuplicateReport = errors.New("duplicate report")

type ReportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{db: db}
}

// Create inserts a new report with status PENDING. A user can only have one
// report per target (unique index in 000009_reports).
func (r *ReportRepository) Create(ctx context.Context, rep *domain.Report) error {
	err := r.db.QueryRow(ctx, `
		INSERT INTO reports (report_id, reporter_id, target_type, target_id, reason, comment, status, created_at)
		VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, 'PENDING', NOW())
		RETURNING report_id, status, created_at
	`,
		rep.ReporterID, rep.TargetType, rep.TargetID, rep.Reason, rep.Comment,
	).Scan(&rep.ID, &rep.Status, &rep.CreatedAt)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrDuplicateReport
	}
	return err
}
