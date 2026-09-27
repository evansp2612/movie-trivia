// Vercel Go function adapter for the backend. Vercel (Root Directory:
// backend) routes every /api/* request to this handler; the heavy init
// (config, Postgres, Redis, TMDB/OMDb clients, repos, usecases, router)
// runs once per function instance via sync.Once — warm instances reuse
// it, cold instances initialize on their first request. Panics surface
// as function errors.
package main

import (
	"net/http"
	"sync"

	"movie-trivia/internal/app"
)

var (
	initOnce sync.Once
	handler  http.Handler
)

// Handler is the Vercel function entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	initOnce.Do(func() {
		a := app.Build(r.Context())
		handler = a.Handler
	})
	handler.ServeHTTP(w, r)
}

// main exists only so `go build ./...` accepts this directory as a
// package main locally; Vercel uses the Handler export instead.
func main() {}
