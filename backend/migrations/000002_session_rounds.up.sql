-- Free Play sessions persist their 10-round variant so every request of
-- the session serves the identical set (daily sessions keep theirs in
-- daily_games and leave this NULL).
ALTER TABLE sessions ADD COLUMN rounds JSONB;
