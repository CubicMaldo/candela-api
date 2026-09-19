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

	"github.com/tuusuario/candela-api/internal/config"
	"github.com/tuusuario/candela-api/internal/infra/postgres"
	redisclient "github.com/tuusuario/candela-api/internal/infra/redis"
	"github.com/tuusuario/candela-api/internal/infra/storage"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
	"github.com/tuusuario/candela-api/internal/web"
	"github.com/tuusuario/candela-api/internal/webdeps"
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

	pool, err := postgres.NewPoolFromConfig(
		ctx,
		cfg.DatabaseURL,
		cfg.DBMaxConns,
		cfg.DBMinConns,
		cfg.DBMaxConnLifetime,
		cfg.DBMaxConnIdleTime,
	)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to PostgreSQL successfully")

	rdb, err := redisclient.NewClient(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Unable to connect to redis: %v", err)
	}
	defer rdb.Close()
	log.Println("Connected to Redis successfully")

	if err := storage.EnsureBucket(
		cfg.StorageEndpoint,
		cfg.StorageAccessKey,
		cfg.StorageSecretKey,
		cfg.StorageBucket,
		cfg.StorageUseSSL,
	); err != nil {
		log.Fatalf("Unable to initialize storage bucket: %v", err)
	}

	store, err := storage.NewMinIO(
		cfg.StorageEndpoint,
		cfg.StoragePublicEndpoint,
		cfg.StorageAccessKey,
		cfg.StorageSecretKey,
		cfg.StorageBucket,
		cfg.StorageUseSSL,
	)
	if err != nil {
		log.Fatalf("Unable to create storage client: %v", err)
	}

	router := web.NewRouter(webdeps.Dependencies{
		Env:     cfg.AppEnv,
		Config:  cfg,
		DBPool:  pool,
		Redis:   rdb,
		Storage: store,
	})

	// Background reclamation of expired photos and refresh tokens.
	janitorCtx, stopJanitor := context.WithCancel(context.Background())
	defer stopJanitor()
	go service.NewJanitor(
		repo.NewCheckInRepository(pool, cfg.LocalTimezone),
		repo.NewRefreshTokenRepository(pool),
		store,
	).Run(janitorCtx)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,  // Mitigates Slowloris by terminating slow header delivery
		ReadTimeout:       15 * time.Second, // Maximum duration for reading the entire request
		WriteTimeout:      30 * time.Second, // Maximum duration before timing out writes of the response
		IdleTimeout:       60 * time.Second, // Maximum amount of time to wait for the next request when keep-alives are enabled
		MaxHeaderBytes:    1 << 20,          // 1 MB maximum header size
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
