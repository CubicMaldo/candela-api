package repo

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tuusuario/candela-api/internal/domain"
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
		INSERT INTO users (email, username, password_hash, birth_date, account_status, privacy_mode)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING user_id, is_protected, points_total, badge_level, created_at, updated_at
	`

	err := r.db.QueryRow(ctx, query,
		u.Email, u.Username, u.PasswordHash, u.BirthDate, u.AccountStatus, u.PrivacyMode,
	).Scan(
		&u.UserID, &u.IsProtected, &u.PointsTotal, &u.BadgeLevel, &u.CreatedAt, &u.UpdatedAt,
	)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
		return ErrUserAlreadyExists
	}
	return err
}

func (r *UserRepository) FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	query := `SELECT user_id, email, username, password_hash, birth_date, is_protected, account_status, privacy_mode, created_at, updated_at FROM users WHERE user_id = $1`

	var u domain.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PasswordHash, &u.BirthDate,
		&u.IsProtected, &u.AccountStatus, &u.PrivacyMode, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// GetByID returns a user including their weekly streak fields and ranking
// opt-in. ranking_opt_in must stay in this projection: the ranking service
// reads it off this struct to decide whether to expose the caller's rank, and
// a missing column silently reads as false for everyone.
func (r *UserRepository) GetByID(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	query := `
		SELECT user_id, email, username, points_total, ranking_opt_in,
		       streak_weeks_current, streak_weeks_max, streak_week_checkins,
		       streak_week_start, streak_last_checkin
		FROM users WHERE user_id = $1
	`

	var u domain.User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PointsTotal, &u.RankingOptIn,
		&u.StreakWeeksCurrent, &u.StreakWeeksMax, &u.StreakWeekCheckins,
		&u.StreakWeekStart, &u.StreakLastCheckin,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// FindByUsername returns a user and their total check-in count.
// Returns ErrUserNotFound if the user does not exist or is not ACTIVE.
func (r *UserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, int, error) {
	query := `
		SELECT u.user_id, u.username, u.badge_level, u.points_total,
		       u.streak_weeks_current, u.streak_weeks_max, u.privacy_mode, u.is_protected,
		       u.account_status, u.ranking_opt_in, u.created_at,
		       COUNT(ci.checkin_id) AS checkin_count
		FROM users u
		LEFT JOIN check_ins ci ON ci.user_id = u.user_id
		WHERE u.username = $1
		  AND u.account_status = 'ACTIVE'
		GROUP BY u.user_id
	`

	var u domain.User
	var checkInCount int
	err := r.db.QueryRow(ctx, query, username).Scan(
		&u.UserID, &u.Username, &u.BadgeLevel, &u.PointsTotal,
		&u.StreakWeeksCurrent, &u.StreakWeeksMax, &u.PrivacyMode, &u.IsProtected,
		&u.AccountStatus, &u.RankingOptIn, &u.CreatedAt,
		&checkInCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, 0, ErrUserNotFound
		}
		return nil, 0, err
	}
	return &u, checkInCount, nil
}

// SearchByUsernamePrefix returns up to `limit` active users whose username
// starts with `prefix` (case-insensitive), for the friends search. Excludes
// the requester, anyone with a blocked relationship with them, and PRIVATE
// users who are not their friends ("nadie te encuentra si no es tu amigo").
// LEAST/GREATEST on uuid matches the Go-side orderedPair (both compare
// bytewise), so the friendships pair lookup is correct.
func (r *UserRepository) SearchByUsernamePrefix(ctx context.Context, requesterID uuid.UUID, prefix string, limit int) ([]domain.UserSearchEntry, error) {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)
	rows, err := r.db.Query(ctx, `
		SELECT u.user_id, u.username, u.badge_level,
		       COALESCE(f.status, '') = 'accepted' AS is_friend
		FROM users u
		LEFT JOIN friendships f
		       ON f.user_id_a = LEAST(u.user_id, $1)
		      AND f.user_id_b = GREATEST(u.user_id, $1)
		WHERE u.username ILIKE $2 || '%'
		  AND u.user_id <> $1
		  AND u.account_status = 'ACTIVE'
		  AND COALESCE(f.status, '') <> 'blocked'
		  AND (u.privacy_mode <> 'PRIVATE' OR COALESCE(f.status, '') = 'accepted')
		ORDER BY u.username ASC
		LIMIT $3
	`, requesterID, escaped, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []domain.UserSearchEntry{}
	for rows.Next() {
		var e domain.UserSearchEntry
		if err := rows.Scan(&e.UserID, &e.Username, &e.BadgeLevel, &e.IsFriend); err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (r *UserRepository) UpdatePrivacyMode(ctx context.Context, userID uuid.UUID, mode string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users SET privacy_mode = $1, updated_at = NOW() WHERE user_id = $2
	`, mode, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateRankingOptIn sets the user's ranking_opt_in flag.
func (r *UserRepository) UpdateRankingOptIn(ctx context.Context, userID uuid.UUID, optIn bool) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE users SET ranking_opt_in = $1, updated_at = NOW() WHERE user_id = $2
	`, optIn, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// DeleteAccount erases the user's content and anonymizes their identity, in one
// transaction. It returns the object-storage keys of the photos it removed so
// the caller can delete the underlying objects.
//
// This honours what the app tells the user ("se eliminarán tu perfil, tus
// check-ins y tu historial"). An earlier version only renamed the account and
// kept every check-in, photo and point transaction.
//
// The users row itself is anonymized rather than deleted: plans and friendships
// created by other people reference it with ON DELETE CASCADE, and removing the
// row would silently destroy *their* history too. What remains carries no
// personal data.
//
// Moderation reports are deliberately kept. They are a safety record about
// conduct, not profile content, and the identity attached to them is already
// anonymized by this same transaction.
func (r *UserRepository) DeleteAccount(ctx context.Context, userID uuid.UUID) ([]string, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var alreadyDeleted bool
	err = tx.QueryRow(ctx, `
		SELECT account_status = 'DELETED' FROM users WHERE user_id = $1 FOR UPDATE
	`, userID).Scan(&alreadyDeleted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	if alreadyDeleted {
		return nil, ErrUserNotFound
	}

	// Collect photo keys before the rows go away.
	rows, err := tx.Query(ctx, `
		SELECT photo_key FROM check_ins
		WHERE user_id = $1 AND photo_key IS NOT NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	var photoKeys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			rows.Close()
			return nil, err
		}
		photoKeys = append(photoKeys, key)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Order matters only where foreign keys are not ON DELETE CASCADE; the
	// explicit statements also keep the intent readable.
	statements := []string{
		`DELETE FROM plan_participants WHERE user_id = $1`,
		`DELETE FROM plans WHERE creator_user_id = $1 OR target_user_id = $1`,
		`DELETE FROM follows WHERE follower_id = $1`,
		`DELETE FROM friendships WHERE user_id_a = $1 OR user_id_b = $1`,
		`DELETE FROM point_transactions WHERE user_id = $1`,
		`DELETE FROM check_ins WHERE user_id = $1`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(ctx, stmt, userID); err != nil {
			return nil, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users SET
			account_status       = 'DELETED',
			email                = 'deleted_' || user_id || '@candela.deleted',
			username             = 'deleted_' || user_id,
			password_hash        = '',
			birth_date           = '1900-01-01',
			guardian_contact     = NULL,
			guardian_token       = NULL,
			points_total         = 0,
			points_season        = 0,
			badge_level          = 'Nuevo en la vuelta',
			streak_weeks_current = 0,
			streak_weeks_max     = 0,
			streak_week_checkins = 0,
			streak_week_start    = NULL,
			streak_last_checkin  = NULL,
			ranking_opt_in       = FALSE,
			privacy_mode         = 'PRIVATE'
		WHERE user_id = $1
	`, userID); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return photoKeys, nil
}

// GetMyProfile returns the authenticated user's full profile with aggregated counts.
// Sensitive fields (password_hash, guardian_token, guardian_contact) are excluded.
func (r *UserRepository) GetMyProfile(ctx context.Context, userID uuid.UUID) (*domain.MyProfile, error) {
	var p domain.MyProfile
	err := r.db.QueryRow(ctx, `
		SELECT
			u.user_id, u.email, u.username, u.birth_date,
			EXTRACT(YEAR FROM age(CURRENT_DATE, u.birth_date))::INTEGER AS age,
			u.is_protected, u.account_status, u.privacy_mode,
			u.points_total, u.points_season, u.badge_level,
			u.streak_weeks_current, u.streak_weeks_max,
			u.streak_week_checkins, u.streak_week_start, u.streak_last_checkin,
			u.ranking_opt_in,
			u.created_at, u.updated_at,
			(SELECT COUNT(*) FROM check_ins ci
			 WHERE ci.user_id = $1 AND ci.validation_passed = true) AS check_in_count,
			(SELECT COUNT(*) FROM friendships f
			 WHERE (f.user_id_a = $1 OR f.user_id_b = $1) AND f.status = 'accepted') AS friends_count,
			(SELECT COUNT(*) FROM plan_participants pp
			 WHERE pp.user_id = $1 AND pp.status = 'COMPLETED') AS completed_plans_count
		FROM users u
		WHERE u.user_id = $1
	`, userID).Scan(
		&p.UserID, &p.Email, &p.Username, &p.BirthDate, &p.Age,
		&p.IsProtected, &p.AccountStatus, &p.PrivacyMode,
		&p.PointsTotal, &p.PointsSeason, &p.BadgeLevel,
		&p.StreakWeeksCurrent, &p.StreakWeeksMax,
		&p.StreakWeekCheckins, &p.StreakWeekStart, &p.StreakLastCheckin,
		&p.RankingOptIn,
		&p.CreatedAt, &p.UpdatedAt,
		&p.CheckInCount, &p.FriendsCount, &p.CompletedPlansCount,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &p, nil
}

// FindByEmail loads the account behind an email for the login flows.
// is_protected must stay in this projection: it becomes a JWT claim that gates
// minor-restricted places, and omitting it would silently sign every login
// token as unprotected.
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `SELECT user_id, email, username, password_hash, birth_date, is_protected, account_status, privacy_mode, created_at, updated_at FROM users WHERE email = $1`

	var u domain.User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&u.UserID, &u.Email, &u.Username, &u.PasswordHash, &u.BirthDate,
		&u.IsProtected, &u.AccountStatus, &u.PrivacyMode, &u.CreatedAt, &u.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &u, nil
}
