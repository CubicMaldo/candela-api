CREATE INDEX idx_check_ins_place_recent
    ON check_ins (place_id, checked_in_at DESC);

CREATE INDEX idx_check_ins_user_recent
    ON check_ins (user_id, checked_in_at DESC);
