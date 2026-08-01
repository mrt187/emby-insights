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

	"github.com/mrt187/EmbyInsights/internal/config"
	"github.com/mrt187/EmbyInsights/internal/server"
	"github.com/mrt187/EmbyInsights/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	if err := store.Migrate(context.Background(), cfg.DatabaseURL); err != nil {
		log.Fatal(err)
	}

	app, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	go app.BackfillPosterImages(context.Background())

	httpServer := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errorsChannel := make(chan error, 1)
	go func() {
		log.Printf("Emby Insights backend listening on %s", cfg.ListenAddress)
		errorsChannel <- httpServer.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-signals:
		context, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(context); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}
}
