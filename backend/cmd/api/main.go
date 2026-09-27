// Command api is the long-running server entry point (local Docker /
// self-hosted): assembles the API stack via internal/app and serves it
// with graceful shutdown. The Vercel adapter lives in api/index.go.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"movie-trivia/internal/app"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env (if present). Real environment variables take precedence,
	// so docker-compose's env_file/environment blocks still win.
	loaded := false
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			loaded = true
			break
		}
	}
	if !loaded {
		log.Printf("config: no .env loaded (falling back to OS environment)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a := app.Build(ctx)

	// ONE daily job (pool refresh → daily game generation → session GC),
	// triggered lazily by the first request of the day — the 00:00 cron
	// ping from the platform boots the function and starts the chain.
	a.Scheduler.RunIfNeeded(ctx)

	srv := &http.Server{
		Addr:              ":" + a.Config.Port,
		Handler:           a.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("api listening on :%s", a.Config.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
