CREATE TABLE IF NOT EXISTS friendships (
    user_id_a  UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    user_id_b  UUID        NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    status     VARCHAR(20) NOT NULL DEFAULT 'pending'
               CHECK (status IN ('pending','accepted','blocked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id_a, user_id_b),
    CHECK (user_id_a < user_id_b)
);

CREATE INDEX IF NOT EXISTS idx_friendships_user_b ON friendships(user_id_b);
CREATE INDEX IF NOT EXISTS idx_friendships_status ON friendships(status);
