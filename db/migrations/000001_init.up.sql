CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    user_id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    email               VARCHAR(255) UNIQUE NOT NULL,
    username            VARCHAR(50)  UNIQUE NOT NULL,
    password_hash       TEXT        NOT NULL,
    birth_date          DATE        NOT NULL,
    age                 INTEGER     NOT NULL DEFAULT 0,
    is_protected        BOOLEAN     NOT NULL DEFAULT false,
    account_status      VARCHAR(20) NOT NULL DEFAULT 'PENDING'
                        CHECK (account_status IN ('PENDING','ACTIVE','SUSPENDED','DELETED')),
    privacy_mode        VARCHAR(10) NOT NULL DEFAULT 'FRIENDS'
                        CHECK (privacy_mode IN ('PUBLIC','FRIENDS','PRIVATE')),
    guardian_contact    VARCHAR(255),
    guardian_verified   BOOLEAN     NOT NULL DEFAULT false,
    guardian_token      UUID        DEFAULT uuid_generate_v4(),
    points_total        INTEGER     NOT NULL DEFAULT 0,
    points_season       INTEGER     NOT NULL DEFAULT 0,
    badge_level         VARCHAR(50) NOT NULL DEFAULT 'Explorador Nuevo',
    streak_current      INTEGER     NOT NULL DEFAULT 0,
    streak_max          INTEGER     NOT NULL DEFAULT 0,
    streak_last_checkin DATE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_status         ON users(account_status);
CREATE INDEX idx_users_is_protected   ON users(is_protected);
CREATE INDEX idx_users_guardian_token ON users(guardian_token);
