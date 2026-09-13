-- name: CreateReport :one
INSERT INTO reports (report_id, reporter_id, target_type, target_id, reason, comment, status, created_at)
VALUES (uuid_generate_v4(), $1, $2, $3, $4, $5, 'PENDING', NOW())
RETURNING report_id, status, created_at;
