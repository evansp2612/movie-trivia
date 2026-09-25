package httphandler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"movie-trivia/internal/usecase"
)

type FreeplayHandler struct{ freeplay *usecase.FreeplayUsecase }

func NewFreeplayHandler(fp *usecase.FreeplayUsecase) *FreeplayHandler {
	return &FreeplayHandler{freeplay: fp}
}

// Start: POST /api/freeplay/start.
func (h *FreeplayHandler) Start(w http.ResponseWriter, r *http.Request) {
	s, err := h.freeplay.Start(r.Context(), playerIDFrom(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// Round: GET /api/freeplay/{id}/round/{n} — {id} is the session PK.
func (h *FreeplayHandler) Round(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{"invalid round index"})
		return
	}
	round, err := h.freeplay.Round(r.Context(), playerIDFrom(r.Context()), r.PathValue("id"), n)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, publicRound(round))
}

// Answer: POST /api/freeplay/{id}/round/{n}/answer.
func (h *FreeplayHandler) Answer(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{"invalid round index"})
		return
	}
	var body struct {
		Guess json.RawMessage `json:"guess"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	s, err := h.freeplay.Answer(r.Context(), playerIDFrom(r.Context()), r.PathValue("id"), n, body.Guess)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// Result: GET /api/freeplay/{id}/result.
func (h *FreeplayHandler) Result(w http.ResponseWriter, r *http.Request) {
	res, err := h.freeplay.Result(r.Context(), playerIDFrom(r.Context()), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
