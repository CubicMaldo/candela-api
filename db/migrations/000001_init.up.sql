CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─── updated_at trigger function ───────────────────────────────────────────
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ─── users ─────────────────────────────────────────────────────────────────
CREATE TABLE users (
    user_id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    email                 VARCHAR(255) UNIQUE NOT NULL,
    username              VARCHAR(50)  UNIQUE NOT NULL,
    password_hash         TEXT        NOT NULL,
    birth_date            DATE        NOT NULL,
    is_protected          BOOLEAN     NOT NULL DEFAULT false,
    account_status        VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                          CHECK (account_status IN ('PENDING','ACTIVE','SUSPENDED','DELETED')),
    privacy_mode          VARCHAR(10) NOT NULL DEFAULT 'FRIENDS'
                          CHECK (privacy_mode IN ('PUBLIC','FRIENDS','PRIVATE')),
    guardian_contact      VARCHAR(255),
    guardian_verified     BOOLEAN     NOT NULL DEFAULT false,
    guardian_token        UUID        DEFAULT uuid_generate_v4(),
    points_total          INTEGER     NOT NULL DEFAULT 0,
    points_season         INTEGER     NOT NULL DEFAULT 0,
    badge_level           VARCHAR(50) NOT NULL DEFAULT 'Nuevo en la vuelta',
    streak_weeks_current  INTEGER     NOT NULL DEFAULT 0,
    streak_weeks_max      INTEGER     NOT NULL DEFAULT 0,
    streak_week_checkins  INTEGER     NOT NULL DEFAULT 0,
    streak_week_start     DATE,
    streak_last_checkin   TIMESTAMPTZ,
    ranking_opt_in        BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_status         ON users(account_status);
CREATE INDEX idx_users_is_protected   ON users(is_protected);
CREATE INDEX idx_users_guardian_token ON users(guardian_token);
CREATE INDEX idx_users_global_ranking
    ON users (points_total DESC)
    WHERE account_status  = 'ACTIVE'
      AND privacy_mode    = 'PUBLIC'
      AND ranking_opt_in  = TRUE;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── point_transactions ────────────────────────────────────────────────────
CREATE TABLE point_transactions (
    tx_id        UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID         NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    points       INTEGER      NOT NULL,
    reason       VARCHAR(20)  NOT NULL
                 CHECK (reason IN ('CHECKIN', 'PLAN', 'REFERRAL')),
    reference_id UUID,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_point_tx_user ON point_transactions(user_id, created_at DESC);

-- ─── businesses ────────────────────────────────────────────────────────────
CREATE TABLE businesses (
    business_id   UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_name    VARCHAR(255) NOT NULL,
    contact_email VARCHAR(255) NOT NULL,
    plan          VARCHAR(20)  NOT NULL DEFAULT 'FREE'
                  CHECK (plan IN ('FREE','BASIC','PREMIUM')),
    is_verified   BOOLEAN     NOT NULL DEFAULT false,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_businesses_updated_at
    BEFORE UPDATE ON businesses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── places ────────────────────────────────────────────────────────────────
CREATE TABLE places (
    place_id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name              VARCHAR(255) NOT NULL,
    description       TEXT,
    category          VARCHAR(20)  NOT NULL
                      CHECK (category IN (
                          'RESTAURANT','CAFE','BAR','NIGHTCLUB',
                          'PARK','STORE','UNIVERSITY','EVENT','OTHER'
                      )),
    minor_suitability VARCHAR(10)  NOT NULL DEFAULT 'PENDING'
                      CHECK (minor_suitability IN ('YES','NO','PENDING')),
    location          GEOGRAPHY(POINT, 4326) NOT NULL,
    geofence_meters   INTEGER     NOT NULL DEFAULT 100
                      CHECK (geofence_meters BETWEEN 50 AND 200),
    business_id       UUID        REFERENCES businesses(business_id) ON DELETE SET NULL,
    is_verified       BOOLEAN     NOT NULL DEFAULT false,
    is_active         BOOLEAN     NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_places_location          ON places USING GIST(location);
CREATE INDEX idx_places_category          ON places(category);
CREATE INDEX idx_places_minor_suitability ON places(minor_suitability);
CREATE INDEX idx_places_active            ON places(is_active);

CREATE TRIGGER trg_places_updated_at
    BEFORE UPDATE ON places
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── check_ins ─────────────────────────────────────────────────────────────
CREATE TABLE check_ins (
    checkin_id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID        NOT NULL REFERENCES users(user_id)  ON DELETE CASCADE,
    place_id            UUID        NOT NULL REFERENCES places(place_id) ON DELETE RESTRICT,
    photo_url           TEXT,
    photo_key           TEXT,
    photo_expires_at    TIMESTAMPTZ,
    photo_exif_ts       TIMESTAMPTZ,
    photo_is_permanent  BOOLEAN     NOT NULL DEFAULT FALSE,
    location            GEOGRAPHY(POINT, 4326) NOT NULL,
    gps_valid           BOOLEAN     NOT NULL DEFAULT false,
    timestamp_valid     BOOLEAN     NOT NULL DEFAULT false,
    validation_passed   BOOLEAN     NOT NULL DEFAULT false,
    points_awarded      INTEGER     NOT NULL DEFAULT 0,
    is_first_of_day     BOOLEAN     NOT NULL DEFAULT false,
    checked_in_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_checkins_checked_in_at ON check_ins(checked_in_at DESC);
CREATE INDEX idx_check_ins_place_recent ON check_ins (place_id, checked_in_at DESC);
CREATE INDEX idx_check_ins_user_recent ON check_ins (user_id, checked_in_at DESC);
CREATE INDEX idx_check_ins_expired_photos ON check_ins (photo_expires_at) WHERE photo_is_permanent = false AND photo_key IS NOT NULL;
CREATE INDEX idx_check_ins_location ON check_ins USING GIST(location);
CREATE UNIQUE INDEX idx_checkins_one_per_place_per_day ON check_ins (user_id, place_id, ((checked_in_at AT TIME ZONE 'America/Bogota')::date)) WHERE validation_passed = true;

-- ─── plans ─────────────────────────────────────────────────────────────────
CREATE TABLE plans (
    plan_id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    creator_user_id  UUID        NOT NULL REFERENCES users(user_id)  ON DELETE CASCADE,
    target_user_id   UUID        REFERENCES users(user_id)  ON DELETE CASCADE,
    place_id         UUID        NOT NULL REFERENCES places(place_id) ON DELETE CASCADE,
    status           VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                     CHECK (status IN ('PENDING', 'ACCEPTED', 'COMPLETED', 'FAILED', 'DECLINED')),
    visibility       VARCHAR(20) NOT NULL DEFAULT 'INVITE'
                     CHECK (visibility IN ('PUBLIC', 'FRIENDS', 'INVITE')),
    message          TEXT,
    deadline_hours   INTEGER     NOT NULL CHECK (deadline_hours IN (24, 48, 72)),
    deadline_at      TIMESTAMPTZ NOT NULL,
    completed_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT no_self_plan CHECK (target_user_id IS NULL OR creator_user_id <> target_user_id)
);
CREATE INDEX idx_plans_target   ON plans(target_user_id, status);
CREATE INDEX idx_plans_creator  ON plans(creator_user_id);
CREATE INDEX idx_plans_deadline ON plans(deadline_at) WHERE status IN ('PENDING','ACCEPTED');

CREATE TRIGGER trg_plans_updated_at
    BEFORE UPDATE ON plans
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ─── plan_participants ─────────────────────────────────────────────────────
CREATE TABLE plan_participants (
    plan_id      UUID        NOT NULL REFERENCES plans(plan_id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    role         VARCHAR(20) NOT NULL
                 CHECK (role IN ('CREATOR', 'GUEST')),
    status       VARCHAR(20) NOT NULL
                 CHECK (status IN ('INVITED', 'JOINED', 'COMPLETED', 'DECLINED')),
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (plan_id, user_id)
);
CREATE INDEX idx_plan_participants_user ON plan_participants(user_id, status);

-- ─── follows ───────────────────────────────────────────────────────────────
CREATE TABLE follows (
    follower_id  UUID        NOT NULL REFERENCES users(user_id)   ON DELETE CASCADE,
    place_id     UUID        NOT NULL REFERENCES places(place_id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, place_id)
);
CREATE INDEX idx_follows_place_id    ON follows (place_id);

-- ─── friendships ───────────────────────────────────────────────────────────
CREATE TABLE friendships (
    user_id_a    UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    user_id_b    UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    requester_id UUID        NOT NULL REFERENCES users(user_id),
    status       VARCHAR(20) NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','accepted','blocked')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id_a, user_id_b),
    CHECK (user_id_a < user_id_b)
);
CREATE INDEX idx_friendships_user_b ON friendships(user_id_b);
CREATE INDEX idx_friendships_status ON friendships(status);

-- ─── reports ───────────────────────────────────────────────────────────────
CREATE TABLE reports (
    report_id    UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    reporter_id  UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    target_type  VARCHAR(20) NOT NULL
                 CHECK (target_type IN ('PHOTO', 'USER', 'PLACE')),
    target_id    UUID        NOT NULL,
    reason       VARCHAR(30) NOT NULL
                 CHECK (reason IN ('INAPPROPRIATE_CONTENT', 'HARASSMENT', 'SPAM', 'FAKE_CONTENT', 'OTHER')),
    comment      TEXT,
    status       VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                 CHECK (status IN ('PENDING', 'REVIEWED', 'DISMISSED')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at  TIMESTAMPTZ,
    reviewed_by  UUID        REFERENCES users(user_id) ON DELETE SET NULL
);
CREATE INDEX idx_reports_status   ON reports (status);
CREATE INDEX idx_reports_target   ON reports (target_type, target_id);
CREATE UNIQUE INDEX idx_reports_unique_per_target ON reports (reporter_id, target_type, target_id);

-- ─── refresh_tokens ────────────────────────────────────────────────────────
CREATE TABLE refresh_tokens (
    token_id    UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    token_hash  BYTEA       NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
