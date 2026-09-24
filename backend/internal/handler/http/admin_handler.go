package httphandler

import (
	"net/http"
	"time"

	"movie-trivia/internal/usecase"
)

type AdminHandler struct{ admin *usecase.AdminUsecase }

func NewAdminHandler(admin *usecase.AdminUsecase) *AdminHandler { return &AdminHandler{admin: admin} }

// Reveal: GET /api/admin/daily/reveal — requires X-Admin-Password.
// Returns today's full round set with correct answers.
func (h *AdminHandler) Reveal(w http.ResponseWriter, r *http.Request) {
	gameDate := r.URL.Query().Get("date")
	if gameDate == "" {
		gameDate = time.Now().Format("2006-01-02")
	}
	rounds, err := h.admin.Reveal(r.Context(), gameDate)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, rounds)
}
