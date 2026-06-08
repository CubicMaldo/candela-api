CREATE TABLE follows (
    follower_id  UUID        NOT NULL REFERENCES users(user_id)   ON DELETE CASCADE,
    place_id     UUID        NOT NULL REFERENCES places(place_id) ON DELETE CASCADE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, place_id)
);

CREATE INDEX idx_follows_place_id    ON follows (place_id);
CREATE INDEX idx_follows_follower_id ON follows (follower_id);
