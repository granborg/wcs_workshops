// Command api runs the WCS Workshops HTTP API and serves the built web app.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/granborg/wcs_workshops/apps/api/internal/auth"
	"github.com/granborg/wcs_workshops/apps/api/internal/config"
	"github.com/granborg/wcs_workshops/apps/api/internal/db"
	"github.com/granborg/wcs_workshops/apps/api/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "vimeo-privacy" {
		os.Exit(runVimeoPrivacy(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "vimeo-downloads" {
		os.Exit(runVimeoDownloads(os.Args[2:]))
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("configuration error", "err", err)
		os.Exit(1)
	}

	database, err := db.Open(cfg.DatabasePath)
	if err != nil {
		log.Error("open database", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Error("migrate database", "err", err)
		os.Exit(1)
	}

	srv := server.New(cfg, database, auth.NewFirebase(cfg.FirebaseProjectID), log)
	hs := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Info("listening", "addr", cfg.Addr, "db", cfg.DatabasePath)
		if host := os.Getenv("FIREBASE_AUTH_EMULATOR_HOST"); host != "" {
			log.Warn("using the Firebase Auth emulator; tokens are not signature-checked", "host", host)
		}
		if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server stopped", "err", err)
			os.Exit(1)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := hs.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "err", err)
	}
}
