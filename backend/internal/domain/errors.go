package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrAlreadyCompleted = errors.New("daily run already completed")
	ErrSessionCompleted = errors.New("session already completed")
	ErrTooManyAttempts  = errors.New("no attempts remaining")
	ErrInvalidName      = errors.New("name must be 1-20 characters")
	ErrNotSubmitted     = errors.New("today's run must be completed before submitting a score")
	ErrDuplicateSubmit  = errors.New("score already submitted for today")
	ErrPoolEmpty        = errors.New("master pool is empty")
	ErrInvalidGuess     = errors.New("invalid guess for this round type")
	ErrGamePreparing    = errors.New("today's game is still being prepared")
)

// ErrNotImplemented marks endpoints whose business logic is still a stub.
var ErrNotImplemented = errors.New("not implemented")
