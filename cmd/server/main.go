package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/tuusuario/nomad-api/internal/config"
	"github.com/tuusuario/nomad-api/internal/infra/postgres"
	redisclient "github.com/tuusuario/nomad-api/internal/infra/redis"
	"github.com/tuusuario/nomad-api/internal/web"
	"github.com/tuusuario/nomad-api/internal/webdeps"

)

func main() {
	if os.Getenv("APP_ENV") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found, using environment variables")
		}
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	rdb, err := redisclient.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Unable to connect to redis: %v", err)
	}
	defer rdb.Close()

	router := web.NewRouter(webdeps.Dependencies{
		Env:    cfg.AppEnv,
		Config: cfg,
		DBPool: pool,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
	}

	go func() {
		log.Printf("Server running on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}
	log.Println("Server stopped cleanly")
}