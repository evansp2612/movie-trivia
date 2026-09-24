package usecase

import (
	"context"
	"crypto/subtle"

	"movie-trivia/internal/repository"
)

// AdminUsecase backs the password-gated reveal page.
type AdminUsecase struct {
	password   string
	dailyGames repository.DailyGameRepo
}

func NewAdminUsecase(password string, dailyGames repository.DailyGameRepo) *AdminUsecase {
	return &AdminUsecase{password: password, dailyGames: dailyGames}
}

// CheckPassword compares the X-Admin-Password header in constant time.
func (u *AdminUsecase) CheckPassword(supplied string) bool {
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(u.password)) == 1
}

// Reveal returns today's full round set including correct answers.
func (u *AdminUsecase) Reveal(ctx context.Context, gameDate string) (any, error) {
	return u.dailyGames.Get(ctx, gameDate)
}
