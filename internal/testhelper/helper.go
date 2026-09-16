package testhelper

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/tuusuario/candela-api/internal/infra/postgres"
	redisclient "github.com/tuusuario/candela-api/internal/infra/redis"
	"github.com/tuusuario/candela-api/internal/infra/storage"
)

// MockStorage implements storage.ObjectStorage with call tracking for testing.
type MockStorage struct {
	mu           sync.Mutex
	DeletedKeys  []string
	StoredFiles  map[string][]byte
	PresignedPut string
	PresignedGet string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		DeletedKeys:  []string{},
		StoredFiles:  make(map[string][]byte),
		PresignedPut: "http://mock-storage/upload",
		PresignedGet: "http://mock-storage/view",
	}
}

func (m *MockStorage) PresignedPutURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return m.PresignedPut + "/" + key, nil
}

func (m *MockStorage) PresignedGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return m.PresignedGet + "/" + key, nil
}

func (m *MockStorage) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeletedKeys = append(m.DeletedKeys, key)
	delete(m.StoredFiles, key)
	return nil
}

func (m *MockStorage) WasDeleted(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range m.DeletedKeys {
		if k == key {
			return true
		}
	}
	return false
}

// GetTestPool connects to PostgreSQL using the environment or localhost fallback.
func GetTestPool(t *testing.T) *pgxpool.Pool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://candela_user:candela_pass_dev@localhost:5432/candela_db?sslmode=disable"
	}

	pool, err := postgres.NewPool(ctx, dbURL)
	if err != nil {
		// Try localhost fallback if postgres host failed
		fallback := "postgres://candela_user:candela_pass_dev@localhost:5432/candela_db?sslmode=disable"
		if pool, err = postgres.NewPool(ctx, fallback); err != nil {
			t.Skipf("Skipping integration test: PostgreSQL not reachable: %v", err)
			return nil
		}
	}

	return pool
}

// GetTestRedis connects to Redis using the environment or localhost fallback.
func GetTestRedis(t *testing.T) *redis.Client {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://:redis_pass_dev@localhost:6379/0"
	}

	rdb, err := redisclient.NewClient(ctx, redisURL)
	if err != nil {
		fallback := "redis://:redis_pass_dev@localhost:6379/0"
		if rdb, err = redisclient.NewClient(ctx, fallback); err != nil {
			t.Skipf("Skipping integration test: Redis not reachable: %v", err)
			return nil
		}
	}

	return rdb
}

// EnsureStorage returns real MinIO storage if available, or mock storage.
func EnsureStorage() storage.ObjectStorage {
	return NewMockStorage()
}
