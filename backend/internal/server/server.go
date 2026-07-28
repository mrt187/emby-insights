package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mrt187/EmbyInsights/internal/config"
	"github.com/redis/go-redis/v9"
)

type App struct {
	database *pgxpool.Pool
	redis    *redis.Client
}

func New(cfg config.Config) (*App, error) {
	database, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	options, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		database.Close()
		return nil, err
	}

	return &App{
		database: database,
		redis:    redis.NewClient(options),
	}, nil
}

func (app *App) Close() {
	app.redis.Close()
	app.database.Close()
}

func (app *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", app.ready)
	return mux
}

func health(writer http.ResponseWriter, _ *http.Request) {
	respondJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (app *App) ready(writer http.ResponseWriter, request *http.Request) {
	context, cancel := context.WithTimeout(request.Context(), 2*time.Second)
	defer cancel()

	if err := app.database.Ping(context); err != nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "database unavailable"})
		return
	}

	if err := app.redis.Ping(context).Err(); err != nil {
		respondJSON(writer, http.StatusServiceUnavailable, map[string]string{"status": "cache unavailable"})
		return
	}

	respondJSON(writer, http.StatusOK, map[string]string{"status": "ready"})
}

func respondJSON(writer http.ResponseWriter, status int, body any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(body)
}
