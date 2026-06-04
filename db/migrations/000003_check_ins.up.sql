CREATE TABLE check_ins (
    checkin_id        UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID        NOT NULL REFERENCES users(user_id)  ON DELETE CASCADE,
    place_id          UUID        NOT NULL REFERENCES places(place_id) ON DELETE RESTRICT,
    photo_url         TEXT,
    photo_expires_at  TIMESTAMPTZ,
    photo_exif_ts     TIMESTAMPTZ,
    gps_lat           FLOAT       NOT NULL,
    gps_lon           FLOAT       NOT NULL,
    gps_valid         BOOLEAN     NOT NULL DEFAULT false,
    timestamp_valid   BOOLEAN     NOT NULL DEFAULT false,
    validation_passed BOOLEAN     NOT NULL DEFAULT false,
    points_awarded    INTEGER     NOT NULL DEFAULT 0,
    is_first_of_day   BOOLEAN     NOT NULL DEFAULT false,
    checked_in_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_checkins_user_id       ON check_ins(user_id);
CREATE INDEX idx_checkins_place_id      ON check_ins(place_id);
CREATE INDEX idx_checkins_checked_in_at ON check_ins(checked_in_at DESC);
-- índice compuesto para detectar duplicados del mismo día
CREATE INDEX idx_checkins_user_place_day
    ON check_ins(user_id, place_id, (checked_in_at::date));
