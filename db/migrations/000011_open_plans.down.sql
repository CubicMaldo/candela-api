DROP TABLE IF EXISTS plan_participants;

ALTER TABLE plans DROP COLUMN IF EXISTS visibility;

ALTER TABLE plans DROP CONSTRAINT IF EXISTS no_self_plan;
ALTER TABLE plans ADD CONSTRAINT no_self_plan
    CHECK (creator_user_id <> target_user_id);
ALTER TABLE plans ALTER COLUMN target_user_id SET NOT NULL;

DROP TYPE IF EXISTS participant_status;
DROP TYPE IF EXISTS participant_role;
DROP TYPE IF EXISTS plan_visibility;
