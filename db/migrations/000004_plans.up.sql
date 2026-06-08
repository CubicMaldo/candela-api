CREATE TYPE plan_status AS ENUM (
    'PENDING', 'ACCEPTED', 'COMPLETED', 'FAILED', 'DECLINED'
);

CREATE TABLE plans (
    plan_id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    creator_user_id  UUID        NOT NULL REFERENCES users(user_id)  ON DELETE CASCADE,
    target_user_id   UUID        NOT NULL REFERENCES users(user_id)  ON DELETE CASCADE,
    place_id         UUID        NOT NULL REFERENCES places(place_id) ON DELETE CASCADE,
    status           plan_status NOT NULL DEFAULT 'PENDING',
    message          TEXT,
    deadline_hours   INTEGER     NOT NULL CHECK (deadline_hours IN (24, 48, 72)),
    deadline_at      TIMESTAMPTZ NOT NULL,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT no_self_plan CHECK (creator_user_id <> target_user_id)
);

CREATE INDEX idx_plans_target   ON plans(target_user_id, status);
CREATE INDEX idx_plans_creator  ON plans(creator_user_id);
-- parcial: solo planes activos necesitan monitoreo de deadline
CREATE INDEX idx_plans_deadline
    ON plans(deadline_at)
    WHERE status IN ('PENDING','ACCEPTED');
