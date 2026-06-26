package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg := loadConfig()
	api := newAPI()
	api.metrics = newAPIMetrics()
	api.cache = openCache(context.Background(), cfg)
	defer closeCache(api.cache)
	db, storageMode, err := openDatabase(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	api.db = db
	api.storageMode = storageMode
	defer closeDatabase(db)

	mux := http.NewServeMux()
	api.register(mux)

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           runtimeMiddleware(mux, cfg, api.metrics),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go api.runWorkers(ctx, cfg)

	errs := make(chan error, 1)
	go func() {
		log.Printf("AI SCRM API listening on http://%s rate_limit=%d/s burst=%d cache_ttl=%s cache_max_entries=%d\n", cfg.Addr, cfg.RateLimitRPS, cfg.RateLimitBurst, cfg.CacheTTL, cfg.CacheMaxEntries)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()

	select {
	case err := <-errs:
		log.Fatal(err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		log.Printf("AI SCRM API shutting down, timeout=%s\n", cfg.ShutdownTimeout)
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Fatal(err)
		}
	}
}
