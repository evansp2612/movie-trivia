package httphandler

import (
	"encoding/json"
	"net/http"

	"movie-trivia/internal/usecase"
)

type LeaderboardHandler struct{ leaderboard *usecase.LeaderboardUsecase }

func NewLeaderboardHandler(lb *usecase.LeaderboardUsecase) *LeaderboardHandler {
	return &LeaderboardHandler{leaderboard: lb}
}

// Top10: GET /api/daily/leaderboard — ordered score DESC, submitted_at ASC.
func (h *LeaderboardHandler) Top10(w http.ResponseWriter, r *http.Request) {
	entries, err := h.leaderboard.Top10(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// Submit: POST /api/daily/leaderboard — one-shot name submission.
func (h *LeaderboardHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{"invalid body"})
		return
	}
	if err := h.leaderboard.Submit(r.Context(), playerIDFrom(r.Context()), body.Name); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "submitted"})
}
