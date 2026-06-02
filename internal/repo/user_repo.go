package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tuusuario/nomad-api/internal/domain"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	query := `
		INSERT INTO users (email, username, password_hash, birth_date, age, account_status, privacy_mode)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING user_id, is_protected, points_total, badge_level, created_at, updated_at
	`
	
	err := r.db.QueryRow(ctx, query,
		u.Email, u.Username, u.PasswordHash, u.BirthDate, u.Age, u.AccountStatus, u.PrivacyMode,
	).Scan(
		&u.UserID, &u.IsProtected, &u.PointsTotal, &u.BadgeLevel, &u.CreatedAt, &u.UpdatedAt,
	)

	return err
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT user_id, email, username, password_hash, birth_date, age, account_status, privacy_mode, created_at, updated_at FROM users WHERE email = $1`
	
	var u domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PasswordHash, &u.BirthDate, &u.Age, 
		&u.AccountStatus, &u.PrivacyMode, &u.CreatedAt, &u.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}
