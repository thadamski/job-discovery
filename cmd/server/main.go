// Command server is the entry point for the job-discovery service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	apihandlers "github.com/thadamski/job-discovery/internal/api"
	"github.com/thadamski/job-discovery/internal/config"
	"github.com/thadamski/job-discovery/internal/events"
	"github.com/thadamski/job-discovery/internal/obs"
	"github.com/thadamski/job-discovery/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	o, shutdownObs, err := obs.New(ctx, cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("initialising observability: %w", err)
	}
	defer shutdownObs()

	log := o.Logger

	st, pool, err := store.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("opening store: %w", err)
	}
	defer pool.Close()

	pub, shutdownPub, err := events.NewPublisher(ctx, cfg.NATSURL)
	if err != nil {
		return fmt.Errorf("creating NATS publisher: %w", err)
	}
	defer shutdownPub()

	appHandler := apihandlers.NewAppHandler(st, pub, o)
	strict := apihandlers.NewStrictHandler(appHandler, nil)

	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(requestIDMiddleware)
	r.Use(slogMiddleware(log))
	r.Use(chimw.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "job-discovery")
	})

	r.Mount("/", apihandlers.Handler(strict))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "postgres not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Handle("/metrics", promhttp.HandlerFor(o.Registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", slog.String("addr", cfg.HTTPAddr))
		if serveErr := srv.ListenAndServe(); serveErr != nil && serveErr != http.ErrServerClosed {
			errCh <- serveErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err = <-errCh:
		return fmt.Errorf("http server: %w", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	return nil
}

// requestIDMiddleware binds the chi request ID into our context helper.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid := chimw.GetReqID(r.Context())
		ctx := apihandlers.WithRequestID(r.Context(), rid)
		w.Header().Set("X-Request-ID", rid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// slogMiddleware logs each request with method, path, status, and duration.
func slogMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			log.InfoContext(
				r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Duration("duration", time.Since(start)),
				slog.String("request_id", apihandlers.RequestIDFromCtx(r.Context())),
			)
		})
	}
}
