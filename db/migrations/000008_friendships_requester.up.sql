TRUNCATE TABLE friendships;
ALTER TABLE friendships ADD COLUMN requester_id UUID NOT NULL REFERENCES users(user_id);
