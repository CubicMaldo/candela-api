-- 000001_init.up.sql
-- Migración inicial — ejecutar con: make migrate-up

-- Extensiones
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ─────────────────────────────────────────
-- BUSINESSES (va antes que PLACES por FK)
-- ─────────────────────────────────────────
CREATE TABLE businesses (
    business_id     UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_name      VARCHAR(255) NOT NULL,
    contact_email   VARCHAR(255) NOT NULL,
    plan            VARCHAR(20)  NOT NULL DEFAULT 'FREE'
                    CHECK (plan IN ('FREE','BASIC','PREMIUM')),
    is_verified     BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─────────────────────────────────────────
-- USERS
-- ─────────────────────────────────────────
CREATE TABLE users (
    user_id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email               VARCHAR(255) UNIQUE NOT NULL,
    username            VARCHAR(50)  UNIQUE NOT NULL,
    password_hash       TEXT NOT NULL,
    birth_date          DATE NOT NULL,
    age                 INTEGER NOT NULL,
    is_protected        BOOLEAN NOT NULL DEFAULT false,
    account_status      VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                        CHECK (account_status IN ('PENDING','ACTIVE','SUSPENDED','DELETED')),
    privacy_mode        VARCHAR(10) NOT NULL DEFAULT 'FRIENDS'
                        CHECK (privacy_mode IN ('PUBLIC','FRIENDS','PRIVATE')),
    -- Parental consent
    guardian_contact    VARCHAR(255),
    guardian_verified   BOOLEAN NOT NULL DEFAULT false,
    guardian_token      UUID DEFAULT uuid_generate_v4(),
    -- Gamification
    points_total        INTEGER NOT NULL DEFAULT 0,
    points_season       INTEGER NOT NULL DEFAULT 0,
    badge_level         VARCHAR(50) NOT NULL DEFAULT 'Explorador Nuevo',
    streak_current      INTEGER NOT NULL DEFAULT 0,
    streak_max          INTEGER NOT NULL DEFAULT 0,
    streak_last_checkin DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_is_protected   ON users(is_protected);
CREATE INDEX idx_users_status         ON users(account_status);
CREATE INDEX idx_users_guardian_token ON users(guardian_token);

-- ─────────────────────────────────────────
-- PLACES
-- ─────────────────────────────────────────
CREATE TABLE places (
    place_id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name                VARCHAR(255) NOT NULL,
    description         TEXT,
    category            VARCHAR(20) NOT NULL
                        CHECK (category IN (
                            'RESTAURANT','CAFE','BAR','NIGHTCLUB',
                            'PARK','STORE','UNIVERSITY','EVENT','OTHER'
                        )),
    minor_suitability   VARCHAR(10) NOT NULL DEFAULT 'PENDING'
                        CHECK (minor_suitability IN ('YES','NO','PENDING')),
    -- PostGIS geography point (lon, lat, SRID 4326)
    location            GEOGRAPHY(POINT, 4326) NOT NULL,
    geofence_meters     INTEGER NOT NULL DEFAULT 100
                        CHECK (geofence_meters BETWEEN 50 AND 200),
    business_id         UUID REFERENCES businesses(business_id) ON DELETE SET NULL,
    is_verified         BOOLEAN NOT NULL DEFAULT false,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_places_location          ON places USING GIST(location);
CREATE INDEX idx_places_minor_suitability ON places(minor_suitability);
CREATE INDEX idx_places_active            ON places(is_active);

-- ─────────────────────────────────────────
-- CHECK-INS
-- ─────────────────────────────────────────
CREATE TABLE check_ins (
    checkin_id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id             UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    place_id            UUID NOT NULL REFERENCES places(place_id),
    -- Foto efímera
    photo_url           TEXT,
    photo_expires_at    TIMESTAMPTZ,
    photo_exif_ts       TIMESTAMPTZ,
    -- Validación GPS
    gps_lat             FLOAT NOT NULL,
    gps_lon             FLOAT NOT NULL,
    gps_valid           BOOLEAN NOT NULL DEFAULT false,
    timestamp_valid     BOOLEAN NOT NULL DEFAULT false,
    validation_passed   BOOLEAN NOT NULL DEFAULT false,
    -- Puntos
    points_awarded      INTEGER NOT NULL DEFAULT 0,
    is_first_of_day     BOOLEAN NOT NULL DEFAULT false,
    checked_in_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_checkins_user_id         ON check_ins(user_id);
CREATE INDEX idx_checkins_place_id        ON check_ins(place_id);
CREATE INDEX idx_checkins_user_place_day    ON check_ins(user_id, place_id, ((checked_in_at AT TIME ZONE 'UTC')::date));
CREATE INDEX idx_checkins_checked_in_at   ON check_ins(checked_in_at DESC);

-- ─────────────────────────────────────────
-- CHALLENGES (RETOS)
-- ─────────────────────────────────────────
CREATE TABLE challenges (
    challenge_id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    creator_user_id         UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    target_user_id          UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    place_id                UUID NOT NULL REFERENCES places(place_id),
    deadline_hours          INTEGER NOT NULL CHECK (deadline_hours IN (24, 48, 72)),
    status                  VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                            CHECK (status IN ('PENDING','ACCEPTED','COMPLETED','FAILED','REJECTED')),
    message                 TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline_at             TIMESTAMPTZ NOT NULL,
    completed_at            TIMESTAMPTZ,
    completing_checkin_id   UUID REFERENCES check_ins(checkin_id),
    points_creator          INTEGER,
    points_target           INTEGER
);

CREATE INDEX idx_challenges_target  ON challenges(target_user_id, status);
CREATE INDEX idx_challenges_creator ON challenges(creator_user_id);
CREATE INDEX idx_challenges_deadline ON challenges(deadline_at) WHERE status IN ('PENDING','ACCEPTED');

-- ─────────────────────────────────────────
-- FOLLOWS (relaciones sociales)
-- Modelo asimétrico: amistad = follow mutuo
-- ─────────────────────────────────────────
CREATE TABLE follows (
    follower_id     UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    following_id    UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, following_id),
    CONSTRAINT no_self_follow CHECK (follower_id <> following_id)
);

CREATE INDEX idx_follows_following ON follows(following_id);

-- ─────────────────────────────────────────
-- POINT TRANSACTIONS (auditoría)
-- ─────────────────────────────────────────
CREATE TABLE point_transactions (
    tx_id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    points          INTEGER NOT NULL,
    reason          VARCHAR(20) NOT NULL
                    CHECK (reason IN (
                        'CHECKIN','BONUS','STREAK',
                        'CHALLENGE','INVITE','ADJUSTMENT'
                    )),
    reference_id    UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_point_tx_user ON point_transactions(user_id, created_at DESC);