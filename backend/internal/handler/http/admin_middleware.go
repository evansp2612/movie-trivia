package httphandler

import (
	"context"
	"net"
	"net/http"
	"time"

	rediscache "movie-trivia/internal/repository/redis"
	"movie-trivia/internal/usecase"

	"github.com/google/uuid"
)

const playerCookie = "player_id"

// contextKey carries the resolved player_id through the request.
type contextKey string

const playerIDKey contextKey = "player_id"

func playerIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(playerIDKey).(string)
	return id
}

func contextWithPlayerID(r *http.Request, playerID string) context.Context {
	return context.WithValue(r.Context(), playerIDKey, playerID)
}

// playerIDMiddleware resolves the anonymous player identity from a
// long-lived HTTP-only cookie, setting one on first visit.
func playerIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		playerID := ""
		if c, err := r.Cookie(playerCookie); err == nil {
			playerID = c.Value
		}
		if playerID == "" {
			playerID = uuid.NewString()
			http.SetCookie(w, &http.Cookie{
				Name:     playerCookie,
				Value:    playerID,
				Path:     "/",
				MaxAge:   int((365 * 24 * time.Hour).Seconds()),
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
		}
		next.ServeHTTP(w, r.WithContext(contextWithPlayerID(r, playerID)))
	})
}

func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Admin-Password")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AdminMiddleware guards the reveal endpoint with X-Admin-Password and
// locks an IP out for 15 minutes after 5 failed attempts.
func AdminMiddleware(admin *usecase.AdminUsecase, lock *rediscache.DailyLock) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			blocked, err := lock.AdminBlocked(r.Context(), ip)
			if err == nil && blocked {
				writeJSON(w, http.StatusTooManyRequests, errorBody{"too many failed attempts; try again later"})
				return
			}
			if !admin.CheckPassword(r.Header.Get("X-Admin-Password")) {
				_, _ = lock.HitAdminFail(r.Context(), ip)
				writeJSON(w, http.StatusUnauthorized, errorBody{"unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
