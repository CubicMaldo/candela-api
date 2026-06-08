CREATE INDEX idx_users_global_ranking
    ON users (points_total DESC)
    WHERE account_status  = 'ACTIVE'
      AND privacy_mode    = 'PUBLIC'
      AND ranking_opt_in  = TRUE;
