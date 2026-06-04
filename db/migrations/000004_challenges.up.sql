CREATE TABLE challenges (
    challenge_id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    creator_user_id       UUID        NOT NULL REFERENCES users(user_id)     ON DELETE CASCADE,
    target_user_id        UUID        NOT NULL REFERENCES users(user_id)     ON DELETE CASCADE,
    place_id              UUID        NOT NULL REFERENCES places(place_id)   ON DELETE RESTRICT,
    deadline_hours        INTEGER     NOT NULL CHECK (deadline_hours IN (24, 48, 72)),
    status                VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                          CHECK (status IN ('PENDING','ACCEPTED','COMPLETED','FAILED','REJECTED')),
    message               TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline_at           TIMESTAMPTZ NOT NULL,
    completed_at          TIMESTAMPTZ,
    completing_checkin_id UUID        REFERENCES check_ins(checkin_id) ON DELETE SET NULL,
    points_creator        INTEGER,
    points_target         INTEGER,
    CONSTRAINT no_self_challenge CHECK (creator_user_id <> target_user_id)
);

CREATE INDEX idx_challenges_target   ON challenges(target_user_id, status);
CREATE INDEX idx_challenges_creator  ON challenges(creator_user_id);
-- parcial: solo retos activos necesitan monitoreo de deadline
CREATE INDEX idx_challenges_deadline
    ON challenges(deadline_at)
    WHERE status IN ('PENDING','ACCEPTED');
