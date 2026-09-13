package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/config"
	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
)

type fakeUserRepo struct {
	byEmail map[string]*domain.User
	created *domain.User
}

func (f *fakeUserRepo) Create(_ context.Context, u *domain.User) error {
	if _, ok := f.byEmail[u.Email]; ok {
		return repo.ErrUserAlreadyExists
	}
	u.UserID = uuid.New()
	u.BadgeLevel = "Nuevo en la vuelta"
	f.created = u
	if f.byEmail == nil {
		f.byEmail = map[string]*domain.User{}
	}
	f.byEmail[u.Email] = u
	return nil
}

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	if u, ok := f.byEmail[email]; ok {
		return u, nil
	}
	return nil, repo.ErrUserNotFound
}

func (f *fakeUserRepo) FindByID(_ context.Context, userID uuid.UUID) (*domain.User, error) {
	for _, u := range f.byEmail {
		if u.UserID == userID {
			return u, nil
		}
	}
	return nil, repo.ErrUserNotFound
}

// fakeRefreshRepo is an in-memory stand-in for the refresh token table.
type fakeRefreshRepo struct {
	// active maps a live token to its owner; rotation removes the old entry so
	// a replayed token fails exactly as it would against the real table.
	active map[string]uuid.UUID
}

func newFakeRefreshRepo() *fakeRefreshRepo {
	return &fakeRefreshRepo{active: map[string]uuid.UUID{}}
}

func (f *fakeRefreshRepo) Create(_ context.Context, userID uuid.UUID, token string, _ time.Time) error {
	f.active[token] = userID
	return nil
}

func (f *fakeRefreshRepo) Rotate(_ context.Context, oldToken, newToken string, _ time.Time) (uuid.UUID, error) {
	userID, ok := f.active[oldToken]
	if !ok {
		return uuid.Nil, repo.ErrRefreshTokenInvalid
	}
	delete(f.active, oldToken)
	f.active[newToken] = userID
	return userID, nil
}

func (f *fakeRefreshRepo) Revoke(_ context.Context, token string) error {
	delete(f.active, token)
	return nil
}

func newTestAuthService(users *fakeUserRepo, validEmail string) *AuthService {
	return &AuthService{
		userRepo:    users,
		refreshRepo: newFakeRefreshRepo(),
		cfg:         config.Config{JWTSecret: "test-secret-at-least-32-characters", GoogleCID: "test-aud"},
		googleValidator: func(_ context.Context, idToken, _ string) (string, error) {
			if idToken == "valid-token" {
				return validEmail, nil
			}
			return "", errors.New("bad token")
		},
	}
}

func TestGoogleLoginExistingUser(t *testing.T) {
	existing := &domain.User{
		UserID:        uuid.New(),
		Email:         "ana@example.com",
		Username:      "ana",
		AccountStatus: "ACTIVE",
	}
	svc := newTestAuthService(&fakeUserRepo{byEmail: map[string]*domain.User{existing.Email: existing}}, existing.Email)

	resp, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "valid-token"})
	if err != nil {
		t.Fatalf("expected login, got error: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected a JWT token")
	}
	if resp.User.Email != existing.Email {
		t.Errorf("expected user %s, got %s", existing.Email, resp.User.Email)
	}
}

func TestGoogleLoginUnregisteredEmailNeedsRegistration(t *testing.T) {
	svc := newTestAuthService(&fakeUserRepo{}, "nueva@example.com")

	_, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "valid-token"})
	var needsReg *GoogleNeedsRegistrationError
	if !errors.As(err, &needsReg) {
		t.Fatalf("expected GoogleNeedsRegistrationError, got %v", err)
	}
	if needsReg.Email != "nueva@example.com" {
		t.Errorf("expected verified email in error, got %q", needsReg.Email)
	}
}

func TestGoogleLoginInvalidToken(t *testing.T) {
	svc := newTestAuthService(&fakeUserRepo{}, "ana@example.com")

	_, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "garbage"})
	if !errors.Is(err, ErrGoogleTokenInvalid) {
		t.Fatalf("expected ErrGoogleTokenInvalid, got %v", err)
	}
}

func TestGoogleRegisterCreatesPasswordlessAccount(t *testing.T) {
	users := &fakeUserRepo{}
	svc := newTestAuthService(users, "nueva@example.com")

	resp, err := svc.GoogleRegister(context.Background(), domain.GoogleRegisterRequest{
		IDToken:   "valid-token",
		Username:  "nueva_user",
		BirthDate: "1995-06-15",
	})
	if err != nil {
		t.Fatalf("expected registration, got error: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected a JWT token")
	}
	if users.created == nil {
		t.Fatal("expected a user to be created")
	}
	if users.created.PasswordHash != "" {
		t.Error("google account must have no password hash")
	}
	if users.created.Email != "nueva@example.com" {
		t.Errorf("email must come from the token, got %q", users.created.Email)
	}
	if users.created.AccountStatus != "ACTIVE" || users.created.PrivacyMode != "FRIENDS" {
		t.Errorf("unexpected defaults: %s / %s", users.created.AccountStatus, users.created.PrivacyMode)
	}
}

func TestGoogleRegisterRejectsMinor(t *testing.T) {
	svc := newTestAuthService(&fakeUserRepo{}, "menor@example.com")

	minorBirth := time.Now().AddDate(-17, 0, 0).Format("2006-01-02")
	_, err := svc.GoogleRegister(context.Background(), domain.GoogleRegisterRequest{
		IDToken:   "valid-token",
		Username:  "menor_user",
		BirthDate: minorBirth,
	})
	if !errors.Is(err, ErrMinorNotSupportedYet) {
		t.Fatalf("expected ErrMinorNotSupportedYet, got %v", err)
	}
}

func TestGoogleRegisterExistingEmailConflicts(t *testing.T) {
	existing := &domain.User{UserID: uuid.New(), Email: "ana@example.com"}
	svc := newTestAuthService(&fakeUserRepo{byEmail: map[string]*domain.User{existing.Email: existing}}, existing.Email)

	_, err := svc.GoogleRegister(context.Background(), domain.GoogleRegisterRequest{
		IDToken:   "valid-token",
		Username:  "otra",
		BirthDate: "1995-06-15",
	})
	if !errors.Is(err, ErrEmailAlreadyUsed) {
		t.Fatalf("expected ErrEmailAlreadyUsed, got %v", err)
	}
}

func TestLoginGoogleOnlyAccountRejected(t *testing.T) {
	googleOnly := &domain.User{
		UserID:       uuid.New(),
		Email:        "ana@example.com",
		PasswordHash: "",
	}
	svc := newTestAuthService(&fakeUserRepo{byEmail: map[string]*domain.User{googleOnly.Email: googleOnly}}, googleOnly.Email)

	_, err := svc.Login(context.Background(), domain.AuthRequest{Email: googleOnly.Email, Password: "whatever"})
	if !errors.Is(err, ErrGoogleOnlyAccount) {
		t.Fatalf("expected ErrGoogleOnlyAccount, got %v", err)
	}
}

func TestRefreshSuccess(t *testing.T) {
	existing := &domain.User{
		UserID:        uuid.New(),
		Email:         "ana@example.com",
		Username:      "ana",
		AccountStatus: "ACTIVE",
	}
	svc := newTestAuthService(&fakeUserRepo{byEmail: map[string]*domain.User{existing.Email: existing}}, existing.Email)

	loginResp, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "valid-token"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if loginResp.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token from initial session")
	}

	refreshResp, err := svc.Refresh(context.Background(), loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("expected successful refresh, got error: %v", err)
	}

	if refreshResp.Token == "" {
		t.Error("expected valid access token after refresh")
	}
	if refreshResp.RefreshToken == "" {
		t.Error("expected new refresh token after refresh")
	}
	if refreshResp.RefreshToken == loginResp.RefreshToken {
		t.Error("refresh token must be rotated to a new value")
	}
	if refreshResp.ExpiresIn != int(accessTokenTTL.Seconds()) {
		t.Errorf("expected expiresIn %d, got %d", int(accessTokenTTL.Seconds()), refreshResp.ExpiresIn)
	}
}

func TestRefreshReplayRejected(t *testing.T) {
	existing := &domain.User{
		UserID:        uuid.New(),
		Email:         "ana@example.com",
		Username:      "ana",
		AccountStatus: "ACTIVE",
	}
	svc := newTestAuthService(&fakeUserRepo{byEmail: map[string]*domain.User{existing.Email: existing}}, existing.Email)

	loginResp, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "valid-token"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// First refresh rotates the token
	_, err = svc.Refresh(context.Background(), loginResp.RefreshToken)
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}

	// Replay attack: attempt to use the consumed token again
	_, err = svc.Refresh(context.Background(), loginResp.RefreshToken)
	if !errors.Is(err, ErrRefreshTokenRejected) {
		t.Fatalf("expected ErrRefreshTokenRejected on replay, got: %v", err)
	}
}

func TestRefreshInactiveAccount(t *testing.T) {
	existing := &domain.User{
		UserID:        uuid.New(),
		Email:         "ana@example.com",
		Username:      "ana",
		AccountStatus: "ACTIVE",
	}
	userRepo := &fakeUserRepo{byEmail: map[string]*domain.User{existing.Email: existing}}
	svc := newTestAuthService(userRepo, existing.Email)

	loginResp, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "valid-token"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Account gets suspended or deleted
	existing.AccountStatus = "SUSPENDED"

	_, err = svc.Refresh(context.Background(), loginResp.RefreshToken)
	if !errors.Is(err, ErrRefreshTokenRejected) {
		t.Fatalf("expected ErrRefreshTokenRejected for suspended user, got: %v", err)
	}
}

func TestLogoutRevokesToken(t *testing.T) {
	existing := &domain.User{
		UserID:        uuid.New(),
		Email:         "ana@example.com",
		Username:      "ana",
		AccountStatus: "ACTIVE",
	}
	svc := newTestAuthService(&fakeUserRepo{byEmail: map[string]*domain.User{existing.Email: existing}}, existing.Email)

	loginResp, err := svc.GoogleLogin(context.Background(), domain.GoogleAuthRequest{IDToken: "valid-token"})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// Logout should revoke the refresh token
	if err := svc.Logout(context.Background(), loginResp.RefreshToken); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// Trying to refresh after logout must fail
	_, err = svc.Refresh(context.Background(), loginResp.RefreshToken)
	if !errors.Is(err, ErrRefreshTokenRejected) {
		t.Fatalf("expected ErrRefreshTokenRejected after logout, got: %v", err)
	}

	// Logout with empty or already revoked token must be idempotent
	if err := svc.Logout(context.Background(), ""); err != nil {
		t.Errorf("expected logout with empty string to succeed, got: %v", err)
	}
	if err := svc.Logout(context.Background(), loginResp.RefreshToken); err != nil {
		t.Errorf("expected idempotent logout to succeed, got: %v", err)
	}
}

