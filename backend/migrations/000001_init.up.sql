-- Durable tables (PRD Part 3 §3). Redis holds all ephemeral state.

CREATE TABLE sessions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id     UUID NOT NULL,
    mode          TEXT NOT NULL CHECK (mode IN ('daily', 'freeplay')),
    current_round INT  NOT NULL DEFAULT 1,
    score         INT  NOT NULL DEFAULT 0,
    is_completed  BOOLEAN NOT NULL DEFAULT FALSE,
    attempts      INT  NOT NULL DEFAULT 0,
    started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    game_date     DATE
);

CREATE INDEX idx_sessions_player_date ON sessions (player_id, game_date);
CREATE INDEX idx_sessions_stale_freeplay ON sessions (started_at) WHERE mode = 'freeplay' AND is_completed = FALSE;

CREATE TABLE daily_games (
    id          SERIAL PRIMARY KEY,
    game_date   DATE NOT NULL UNIQUE,
    rounds      JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE leaderboard_entries (
    id           BIGSERIAL PRIMARY KEY,
    game_date    DATE NOT NULL,
    player_id    UUID NOT NULL,
    name         VARCHAR(20) NOT NULL,
    score        INT NOT NULL,
    submitted_at TIMESTAMPTZ NOT NULL,
    -- One submission per player per day.
    UNIQUE (game_date, player_id)
);

CREATE INDEX idx_leaderboard_rank ON leaderboard_entries (game_date, score DESC, submitted_at ASC);

CREATE TABLE player_daily_status (
    id           BIGSERIAL PRIMARY KEY,
    player_id    UUID NOT NULL,
    game_date    DATE NOT NULL,
    is_completed BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (game_date, player_id)
);
