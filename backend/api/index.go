// Vercel Go function adapter for the backend. Vercel (Root Directory:
// backend) routes every /api/* request to this handler; the heavy init
// (config, Postgres, Redis, TMDB/OMDb clients, repos, usecases, router)
// runs once per function instance via sync.Once — warm instances reuse
// it, cold instances initialize on their first request. Panics surface
// as function errors.
//
// Package name: Vercel's Go builder requires `package handler` (with the
// exported Handler function) for this function shape — `package main`
// is rejected at build time. Locally it compiles as an ordinary
// library package; only Vercel's generated wrapper calls it.
package handler

import (
	"net/http"
	"sync"

	"movie-trivia/internal/app"
)

var (
	initOnce   sync.Once
	appHandler http.Handler
)

// Handler is the Vercel function entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		a := app.Build(r.Context())
		appHandler = a.Handler
	})
	appHandler.ServeHTTP(w, r)
}
