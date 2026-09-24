# movie-trivia

A fast-paced, spoiler-free movie trivia game (Game of the Day + Free Play) built on a Go API and a static frontend. All game logic, dataset generation, and scoring live on the backend; the frontend is UI-only.

## Game modes

- **Game of the Day** — one globally fixed 10-round set per calendar day, resumable until completed. Daily leaderboard (top 10, one submission per player).
- **Free Play** — endless random 10-round sessions, no leaderboards.

**Question types** (strict 3-3-4 "shuffled bag" per game):

1. **Higher or Lower** — guess which of two movies has the higher IMDb rating (10 pts).
2. **Blurred Poster** — identify a textless poster from an auto-complete search; 5 attempts, 10/8/6/4/2 pts.
3. **Guess the Year** — slider guess of the release year, −1 pt per year off, floor at 0.

Players are identified by an anonymous UUID stored in a long-lived HTTP-only cookie.

## Layout

```
backend/                 Go API
├── cmd/api/             wiring: config, postgres, redis, clients, router
├── internal/
│   ├── config/          env configuration (TMDB/OMDB keys, URLs, TZ)
│   ├── cron/            background jobs: 4h master-pool refresh, daily game generation
│   ├── domain/          models, scoring rules, errors
│   ├── handler/http/    REST routes, player-cookie + admin middleware
│   ├── provider/        TMDB and OMDb API clients
│   ├── repository/      Postgres (durable) + Redis (cache/locks) implementations
│   └── usecase/         game orchestration, shuffled-bag logic, use case tests
└── migrations/          Postgres schema (sessions, daily_games, leaderboard_entries, player_daily_status)

frontend/                Static UI (landing, game, end screen, admin reveal), nginx in Docker
docker-compose.yml       postgres + redis + api + web
```

## API

| Endpoint | Purpose |
|---|---|
| `GET /api/pool/titles` | Movie titles for the auto-complete search |
| `GET /api/daily/status` | Today's completion state + top 10 |
| `POST /api/daily/start` | Start or resume today's run (409 once completed) |
| `GET /api/daily/round/{n}` | Round payload |
| `POST /api/daily/round/{n}/answer` | Submit a guess |
| `GET /api/daily/result` | Final score + breakdown |
| `POST /api/daily/leaderboard` | One-shot name submission |
| `GET /api/daily/leaderboard` | Top 10 (score DESC, submitted_at ASC) |
| `POST /api/freeplay/start` | Start a random run |
| `GET/POST /api/freeplay/{id}/round/{n}[/answer]` | Free Play rounds |
| `GET /api/freeplay/{id}/result` | Free Play final score |
| `GET /api/admin/daily/reveal` | Admin-only reveal (`X-Admin-Password` header) |

## Run

1. Copy `backend/.env.example` to `backend/.env` and fill in `TMDB_API_KEY` and `OMDB_API_KEY`.
2. Start everything:

```bash
docker compose up --build
```

- Frontend: http://localhost:3000
- API: http://localhost:8080 (also proxied at http://localhost:3000/api/)

Local Postgres + Redis only:

```bash
docker compose up postgres redis
cd backend
cp .env.example .env   # fill in API keys
go run ./cmd/api
```

## Tests

```bash
cd backend
go test ./...
```
