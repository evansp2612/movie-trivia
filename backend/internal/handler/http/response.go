package httphandler

import (
	"encoding/json"
	"errors"
	"net/http"

	"movie-trivia/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorBody struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{err.Error()})
	case errors.Is(err, domain.ErrAlreadyCompleted):
		writeJSON(w, http.StatusConflict, errorBody{err.Error()})
	case errors.Is(err, domain.ErrSessionCompleted):
		writeJSON(w, http.StatusConflict, errorBody{err.Error()})
	case errors.Is(err, domain.ErrInvalidName):
		writeJSON(w, http.StatusBadRequest, errorBody{err.Error()})
	case errors.Is(err, domain.ErrInvalidGuess):
		writeJSON(w, http.StatusBadRequest, errorBody{err.Error()})
	case errors.Is(err, domain.ErrNotSubmitted):
		writeJSON(w, http.StatusForbidden, errorBody{err.Error()})
	case errors.Is(err, domain.ErrDuplicateSubmit):
		writeJSON(w, http.StatusConflict, errorBody{err.Error()})
	case errors.Is(err, domain.ErrPoolEmpty):
		writeJSON(w, http.StatusServiceUnavailable, errorBody{err.Error()})
	case errors.Is(err, domain.ErrGamePreparing):
		w.Header().Set("Retry-After", "5")
		writeJSON(w, http.StatusServiceUnavailable, errorBody{err.Error()})
	case errors.Is(err, domain.ErrNotImplemented):
		writeJSON(w, http.StatusNotImplemented, errorBody{err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, errorBody{"internal error"})
	}
}
