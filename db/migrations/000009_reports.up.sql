CREATE TYPE report_target_type AS ENUM ('PHOTO', 'USER', 'PLACE');
CREATE TYPE report_reason AS ENUM (
    'INAPPROPRIATE_CONTENT', 'HARASSMENT', 'SPAM', 'FAKE_CONTENT', 'OTHER'
);
CREATE TYPE report_status AS ENUM ('PENDING', 'REVIEWED', 'DISMISSED');

CREATE TABLE reports (
    report_id    UUID               PRIMARY KEY DEFAULT uuid_generate_v4(),
    reporter_id  UUID               NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    target_type  report_target_type NOT NULL,
    target_id    UUID               NOT NULL,
    reason       report_reason      NOT NULL,
    comment      TEXT,
    status       report_status      NOT NULL DEFAULT 'PENDING',
    created_at   TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    reviewed_at  TIMESTAMPTZ,
    reviewed_by  UUID               REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE INDEX idx_reports_status   ON reports (status);
CREATE INDEX idx_reports_target   ON reports (target_type, target_id);
CREATE INDEX idx_reports_reporter ON reports (reporter_id);
