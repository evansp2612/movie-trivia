package domain

import "fmt"

type RoundType string

const (
	RoundHigherLower RoundType = "higher_lower"
	RoundBlurred     RoundType = "blurred_poster"
	RoundGuessYear   RoundType = "guess_the_year"
)

// RoundsPerGame is fixed for both modes; the "Shuffled Bag" yields a
// strict 3-3-4 distribution with no type repeating more than twice in a row.
const (
	RoundsPerGame    = 10
	BlurredMaxTries  = 5
	GuessYearMaxDiff = 10
	PointsPerRound   = 10
)

type Round struct {
	Index  int       `json:"index"` // 1-based
	Type   RoundType `json:"type"`
	Movies []Movie   `json:"movies"` // 2 for higher/lower, 1 otherwise
	// YearMin/YearMax bound the Guess the Year slider to the pool's
	// release-year range.
	YearMin int `json:"year_min,omitempty"`
	YearMax int `json:"year_max,omitempty"`
}

// ScoreForRound maps an outcome to points: 10 for a correct higher/lower
// or exact year, 10/8/6/4/2 by attempt for the blurred poster, floored at 0.
func ScoreForRound(rt RoundType, correct bool, attempt, yearDiff int) (int, error) {
	switch rt {
	case RoundHigherLower:
		if correct {
			return PointsPerRound, nil
		}
		return 0, nil
	case RoundBlurred:
		if !correct || attempt < 1 || attempt > BlurredMaxTries {
			return 0, nil
		}
		return PointsPerRound - 2*(attempt-1), nil
	case RoundGuessYear:
		if yearDiff < 0 {
			return 0, fmt.Errorf("negative year diff")
		}
		if yearDiff >= GuessYearMaxDiff {
			return 0, nil
		}
		return PointsPerRound - yearDiff, nil
	default:
		return 0, fmt.Errorf("unknown round type %q", rt)
	}
}
