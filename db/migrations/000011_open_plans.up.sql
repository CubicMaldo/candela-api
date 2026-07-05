-- Open plans: a plan no longer requires a single invited target. Plans gain a
-- visibility (PUBLIC / FRIENDS / INVITE) and become multi-participant via a new
-- plan_participants table. INVITE plans keep the 1:1 targeted behaviour and set
-- target_user_id; PUBLIC/FRIENDS plans leave it NULL and gather participants
-- who join later.

CREATE TYPE plan_visibility AS ENUM ('PUBLIC', 'FRIENDS', 'INVITE');
CREATE TYPE participant_role AS ENUM ('CREATOR', 'GUEST');
CREATE TYPE participant_status AS ENUM ('INVITED', 'JOINED', 'COMPLETED', 'DECLINED');

-- target_user_id becomes optional (only INVITE plans carry one).
ALTER TABLE plans ALTER COLUMN target_user_id DROP NOT NULL;

-- The old CHECK(creator <> target) rejects NULL comparisons ambiguously; replace
-- it with one that only constrains the INVITE case (target present).
ALTER TABLE plans DROP CONSTRAINT IF EXISTS no_self_plan;
ALTER TABLE plans ADD CONSTRAINT no_self_plan
    CHECK (target_user_id IS NULL OR creator_user_id <> target_user_id);

ALTER TABLE plans
    ADD COLUMN visibility plan_visibility NOT NULL DEFAULT 'INVITE';

CREATE TABLE plan_participants (
    plan_id      UUID               NOT NULL REFERENCES plans(plan_id) ON DELETE CASCADE,
    user_id      UUID               NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    role         participant_role   NOT NULL,
    status       participant_status NOT NULL,
    joined_at    TIMESTAMPTZ        NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (plan_id, user_id)
);

-- "my plans" lookups (incoming/outgoing/history) filter by participant + status.
CREATE INDEX idx_plan_participants_user ON plan_participants(user_id, status);
CREATE INDEX idx_plan_participants_plan ON plan_participants(plan_id);
