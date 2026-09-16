package testhelper

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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

// CreateTestUser inserts a clean test user into the test database and registers cleanup.
func CreateTestUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, isProtected bool) uuid.UUID {
	userID := uuid.New()
	email := fmt.Sprintf("test-%s@example.com", userID.String()[:8])
	username := fmt.Sprintf("user_%s", userID.String()[:8])

	birthDate := "2000-01-01"
	if isProtected {
		birthDate = time.Now().AddDate(-16, 0, 0).Format("2006-01-02")
	}

	_, err := pool.Exec(ctx, `
		INSERT INTO users (
			user_id, email, username, password_hash, birth_date,
			is_protected, account_status, privacy_mode, ranking_opt_in
		) VALUES ($1, $2, $3, 'hash123', $4, $5, 'ACTIVE', 'PUBLIC', true)
	`, userID, email, username, birthDate, isProtected)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM plans WHERE creator_user_id = $1 OR target_user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM friendships WHERE user_id_a = $1 OR user_id_b = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM point_transactions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM check_ins WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE user_id = $1`, userID)
	})

	return userID
}

// MakeFriends creates an accepted friendship between userA and userB.
func MakeFriends(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userA, userB uuid.UUID) {
	idA, idB := userA, userB
	if idA.String() > idB.String() {
		idA, idB = idB, idA
	}
	_, err := pool.Exec(ctx, `
		INSERT INTO friendships (user_id_a, user_id_b, requester_id, status, created_at)
		VALUES ($1, $2, $3, 'accepted', NOW())
		ON CONFLICT (user_id_a, user_id_b) DO UPDATE SET status = 'accepted'
	`, idA, idB, userA)
	if err != nil {
		t.Fatalf("failed to make friends: %v", err)
	}
}

// GenerateTestJWT mints a signed JWT string for testing auth middleware.
func GenerateTestJWT(secret string, userID uuid.UUID, accountStatus string, isProtected bool, expiry time.Time) string {
	claims := jwt.MapClaims{
		"sub":            userID.String(),
		"email":          "test@example.com",
		"account_status": accountStatus,
		"is_protected":   isProtected,
		"exp":            expiry.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(secret))
	return signed
}
