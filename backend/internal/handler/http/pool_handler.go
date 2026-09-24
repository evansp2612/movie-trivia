package httphandler

import (
	"net/http"

	"movie-trivia/internal/usecase"
)

type PoolHandler struct{ pool *usecase.PoolUsecase }

func NewPoolHandler(pool *usecase.PoolUsecase) *PoolHandler { return &PoolHandler{pool: pool} }

// Titles: GET /api/pool/titles — lightweight list powering the
// frontend auto-complete search against the unified master pool.
func (h *PoolHandler) Titles(w http.ResponseWriter, r *http.Request) {
	titles, err := h.pool.Titles(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"titles": titles})
}
