package httphandler

import (
	"context"
	"net/http"
)

// ForceRunner runs the daily maintenance chain synchronously. Satisfied
// by an adapter over cron.Scheduler in main.go (the interface keeps the
// handler package free of the cron import).
type ForceRunner interface {
	ForceRun(ctx context.Context) (any, error)
}

type CronHandler struct {
	runner    ForceRunner
	secret    string // CRON_SECRET: Vercel Cron sends it as a Bearer token
	adminPass string // ADMIN_PASSWORD: also accepted via X-Admin-Password
}

func NewCronHandler(runner ForceRunner, cronSecret, adminPassword string) *CronHandler {
	return &CronHandler{runner: runner, secret: cronSecret, adminPass: adminPassword}
}

// Daily: GET /api/cron/daily — the platform cron's trigger. Runs the
// full daily maintenance chain synchronously and reports what it did.
//
// Auth (either):
//   - X-Admin-Password: <ADMIN_PASSWORD>   (cron-job.org custom header)
//   - Authorization: Bearer <CRON_SECRET>  (Vercel Cron sends this
//     automatically when the CRON_SECRET env var is set on the project)
func (h *CronHandler) Daily(w http.ResponseWriter, r *http.Request) {
	if !h.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, errorBody{"unauthorized"})
		return
	}
	summary, err := h.runner.ForceRun(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (h *CronHandler) authorized(r *http.Request) bool {
	if h.adminPass != "" && r.Header.Get("X-Admin-Password") == h.adminPass {
		return true
	}
	if h.secret == "" {
		return false
	}
	const prefix = "Bearer "
	auth := r.Header.Get("Authorization")
	return len(auth) > len(prefix) && auth[:len(prefix)] == prefix && auth[len(prefix):] == h.secret
}
