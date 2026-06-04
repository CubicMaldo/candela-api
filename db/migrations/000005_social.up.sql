CREATE TABLE point_transactions (
    tx_id        UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id      UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    points       INTEGER     NOT NULL,
    reason       VARCHAR(20) NOT NULL
                 CHECK (reason IN ('CHECKIN','BONUS','STREAK','CHALLENGE','INVITE','ADJUSTMENT')),
    reference_id UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_point_tx_user ON point_transactions(user_id, created_at DESC);

-- ─────────────────────────────────────────────────────────────
-- Friendships: relación simétrica, PRIMARY KEY garantiza
-- que cada par aparece una sola vez (user_id_a < user_id_b)
-- ─────────────────────────────────────────────────────────────
CREATE TABLE friendships (
    user_id_a  UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    user_id_b  UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status     VARCHAR(20) NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending','accepted','blocked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id_a, user_id_b),
    CHECK (user_id_a < user_id_b)
);

CREATE INDEX idx_friendships_user_b ON friendships(user_id_b);
CREATE INDEX idx_friendships_status ON friendships(status);
