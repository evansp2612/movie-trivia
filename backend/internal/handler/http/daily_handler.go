package httphandler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"movie-trivia/internal/usecase"
)

type DailyHandler struct{ daily *usecase.DailyUsecase }

func NewDailyHandler(daily *usecase.DailyUsecase) *DailyHandler { return &DailyHandler{daily: daily} }

// Status: GET /api/daily/status — completion state + today's top 10.
func (h *DailyHandler) Status(w http.ResponseWriter, r *http.Request) {
	completed, err := h.daily.Status(r.Context(), playerIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	top, err := h.daily.Top10(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"is_completed": completed, "leaderboard": top})
}

// Start: POST /api/daily/start — start or resume; 409 once completed.
func (h *DailyHandler) Start(w http.ResponseWriter, r *http.Request) {
	s, err := h.daily.Start(r.Context(), playerIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// Round: GET /api/daily/round/{n}.
func (h *DailyHandler) Round(w http.ResponseWriter, r *http.Request) {
	n, err := roundIndex(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{"invalid round index"})
		return
	}
	round, err := h.daily.Round(r.Context(), playerIDFrom(r.Context()), n)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, publicRound(round))
}

// Answer: POST /api/daily/round/{n}/answer.
func (h *DailyHandler) Answer(w http.ResponseWriter, r *http.Request) {
	n, err := roundIndex(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{"invalid round index"})
		return
	}
	var body struct {
		Guess json.RawMessage `json:"guess"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	s, err := h.daily.Answer(r.Context(), playerIDFrom(r.Context()), n, body.Guess)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// Result: GET /api/daily/result.
func (h *DailyHandler) Result(w http.ResponseWriter, r *http.Request) {
	res, err := h.daily.Result(r.Context(), playerIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func roundIndex(r *http.Request) (int, error) {
	return strconv.Atoi(r.PathValue("n"))
}
