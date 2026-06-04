package main

import (
	"errors"
	"log"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/joho/godotenv"
)

func main() {
	if os.Getenv("APP_ENV") != "production" {
		_ = godotenv.Load()
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	m, err := migrate.New("file://db/migrations", dbURL)
	if err != nil {
		log.Fatalf("migrate.New: %v", err)
	}
	defer m.Close()

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate up: %v", err)
		}
		log.Println("Migrations applied successfully")

	case "down":
		steps := 1
		if len(os.Args) > 2 {
			if n, err := strconv.Atoi(os.Args[2]); err == nil {
				steps = n
			}
		}
		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down: %v", err)
		}
		log.Printf("Rolled back %d migration(s)", steps)

	case "drop":
		if err := m.Drop(); err != nil {
			log.Fatalf("migrate drop: %v", err)
		}
		log.Println("All tables dropped")

	default:
		log.Fatalf("unknown command: %s (use: up | down [N] | drop)", cmd)
	}
}
