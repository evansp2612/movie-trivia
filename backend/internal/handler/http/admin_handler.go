package httphandler

import (
	"net/http"
	"time"

	"movie-trivia/internal/usecase"
)

type AdminHandler struct {
	admin *usecase.AdminUsecase
	// loc is the game-day timezone (POOL_REFRESH_TZ): "today" for the
	// reveal must match the calendar day players are on, not the
	// server's local/UTC date.
	loc *time.Location
}

func NewAdminHandler(admin *usecase.AdminUsecase, loc *time.Location) *AdminHandler {
	return &AdminHandler{admin: admin, loc: loc}
}

// Reveal: GET /api/admin/daily/reveal — requires X-Admin-Password.
// Returns today's full round set with correct answers. An explicit
// ?date=YYYY-MM-DD overrides today.
func (h *AdminHandler) Reveal(w http.ResponseWriter, r *http.Request) {
	gameDate := r.URL.Query().Get("date")
	if gameDate == "" {
		gameDate = time.Now().In(h.loc).Format("2006-01-02")
	}
	rounds, err := h.admin.Reveal(r.Context(), gameDate)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rounds)
}
