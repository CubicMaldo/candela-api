package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"

	"github.com/tuusuario/candela-api/internal/config"
	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
)

var (
	ErrInvalidCredentials   = errors.New("Correo o contraseña incorrectos.")
	ErrMinorNotSupportedYet = errors.New("Pronto Candela estará disponible para ti.")
	ErrInvalidBirthDate     = errors.New("birth_date must be in YYYY-MM-DD format")
	ErrGoogleTokenInvalid   = errors.New("No pudimos validar tu cuenta de Google. Inténtalo de nuevo.")
	ErrGoogleOnlyAccount    = errors.New("Esta cuenta entra con Google. Usa el botón de Google.")
	ErrEmailAlreadyUsed     = errors.New("Este correo o nombre de usuario ya está en uso.")
	ErrRefreshTokenRejected = errors.New("Tu sesión expiró. Vuelve a entrar.")
)

// GoogleNeedsRegistrationError signals that the Google token is valid but the
// email has no account yet; the handler turns it into a needs_registration
// response instead of an error.
type GoogleNeedsRegistrationError struct {
	Email string
}

func (e *GoogleNeedsRegistrationError) Error() string {
	return "google account not registered"
}

// GoogleTokenValidator verifies a Google ID token against the audience and
// returns the verified email. Injectable so tests can fake Google.
type GoogleTokenValidator func(ctx context.Context, idToken, audience string) (string, error)

func defaultGoogleValidator(ctx context.Context, idToken, audience string) (string, error) {
	payload, err := idtoken.Validate(ctx, idToken, audience)
	if err != nil {
		return "", err
	}
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return "", errors.New("email claim missing")
	}
	return email, nil
}

// accessTokenTTL is short by design: the JWT carries claims (account status,
// protection flag) that cannot be revoked mid-flight, so it must age out fast.
// The refresh token is what keeps the user signed in.
const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 60 * 24 * time.Hour
)

// AuthUserRepo is the slice of UserRepository the auth service needs.
// *repo.UserRepository satisfies it; tests can substitute a fake.
type AuthUserRepo interface {
	Create(ctx context.Context, u *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, userID uuid.UUID) (*domain.User, error)
}

// AuthRefreshRepo is the slice of RefreshTokenRepository the auth service needs.
type AuthRefreshRepo interface {
	Create(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	Rotate(ctx context.Context, oldToken, newToken string, newExpiry time.Time) (uuid.UUID, error)
	Revoke(ctx context.Context, token string) error
}

type AuthService struct {
	userRepo        AuthUserRepo
	refreshRepo     AuthRefreshRepo
	cfg             config.Config
	googleValidator GoogleTokenValidator
}

func NewAuthService(ur AuthUserRepo, rr AuthRefreshRepo, cfg config.Config) *AuthService {
	return NewAuthServiceWithCustom(ur, rr, cfg, defaultGoogleValidator)
}

func NewAuthServiceWithCustom(ur AuthUserRepo, rr AuthRefreshRepo, cfg config.Config, gv GoogleTokenValidator) *AuthService {
	return &AuthService{
		userRepo:        ur,
		refreshRepo:     rr,
		cfg:             cfg,
		googleValidator: gv,
	}
}

// newRefreshToken returns 256 bits of cryptographically random URL-safe text.
func newRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// issueSession mints an access/refresh pair for a freshly authenticated user.
func (s *AuthService) issueSession(ctx context.Context, user *domain.User) (*domain.AuthResponse, error) {
	accessToken, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	refreshToken, err := newRefreshToken()
	if err != nil {
		return nil, err
	}
	if err := s.refreshRepo.Create(ctx, user.UserID, refreshToken, time.Now().UTC().Add(refreshTokenTTL)); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &domain.AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
		User:         *user,
	}, nil
}

// Refresh exchanges a valid refresh token for a new access/refresh pair. The
// old token is consumed in the same transaction that issues the new one, so a
// stolen token stops working the moment the real client next refreshes.
//
// Claims are rebuilt from the current database row, not copied from the
// expiring token: a suspension or privacy change takes effect on the next
// refresh instead of lingering for the life of the old claims.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (*domain.AuthResponse, error) {
	newToken, err := newRefreshToken()
	if err != nil {
		return nil, err
	}

	userID, err := s.refreshRepo.Rotate(ctx, refreshToken, newToken, time.Now().UTC().Add(refreshTokenTTL))
	if err != nil {
		if errors.Is(err, repo.ErrRefreshTokenInvalid) {
			return nil, ErrRefreshTokenRejected
		}
		return nil, err
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, ErrRefreshTokenRejected
	}
	if user.AccountStatus != "ACTIVE" {
		return nil, ErrRefreshTokenRejected
	}

	accessToken, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{
		Token:        accessToken,
		RefreshToken: newToken,
		ExpiresIn:    int(accessTokenTTL.Seconds()),
		User:         *user,
	}, nil
}

// Logout revokes a single refresh token. Idempotent: logging out twice, or with
// a token that was already invalid, still succeeds.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return nil
	}
	return s.refreshRepo.Revoke(ctx, refreshToken)
}

func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResponse, error) {
	birthDate, err := time.Parse("2006-01-02", req.BirthDate)
	if err != nil {
		return nil, ErrInvalidBirthDate
	}

	tempUser := &domain.User{BirthDate: birthDate}
	if tempUser.CalculateAge(time.Now()) < 18 {
		return nil, ErrMinorNotSupportedYet
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:         req.Email,
		Username:      req.Username,
		PasswordHash:  string(hash),
		BirthDate:     birthDate,
		IsProtected:   false,
		AccountStatus: "ACTIVE",
		PrivacyMode:   "FRIENDS",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repo.ErrUserAlreadyExists) {
			return nil, ErrEmailAlreadyUsed
		}
		return nil, err
	}

	return s.issueSession(ctx, user)
}

func (s *AuthService) Login(ctx context.Context, req domain.AuthRequest) (*domain.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if err == repo.ErrUserNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	// Accounts created via Google registration have no password.
	if user.PasswordHash == "" {
		return nil, ErrGoogleOnlyAccount
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	return s.issueSession(ctx, user)
}

func (s *AuthService) GoogleLogin(ctx context.Context, req domain.GoogleAuthRequest) (*domain.AuthResponse, error) {
	email, err := s.googleValidator(ctx, req.IDToken, s.cfg.GoogleCID)
	if err != nil {
		return nil, ErrGoogleTokenInvalid
	}

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if err == repo.ErrUserNotFound {
			return nil, &GoogleNeedsRegistrationError{Email: email}
		}
		return nil, err
	}

	return s.issueSession(ctx, user)
}

// GoogleRegister creates an account for a verified Google email that has none
// yet. The account has no password — it always signs in with Google.
func (s *AuthService) GoogleRegister(ctx context.Context, req domain.GoogleRegisterRequest) (*domain.AuthResponse, error) {
	email, err := s.googleValidator(ctx, req.IDToken, s.cfg.GoogleCID)
	if err != nil {
		return nil, ErrGoogleTokenInvalid
	}

	if _, err := s.userRepo.FindByEmail(ctx, email); err == nil {
		return nil, ErrEmailAlreadyUsed
	} else if err != repo.ErrUserNotFound {
		return nil, err
	}

	birthDate, err := time.Parse("2006-01-02", req.BirthDate)
	if err != nil {
		return nil, ErrInvalidBirthDate
	}

	tempUser := &domain.User{BirthDate: birthDate}
	if tempUser.CalculateAge(time.Now()) < 18 {
		return nil, ErrMinorNotSupportedYet
	}

	user := &domain.User{
		Email:         email,
		Username:      req.Username,
		PasswordHash:  "", // Google-only account
		BirthDate:     birthDate,
		IsProtected:   false,
		AccountStatus: "ACTIVE",
		PrivacyMode:   "FRIENDS",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repo.ErrUserAlreadyExists) {
			return nil, ErrEmailAlreadyUsed
		}
		return nil, err
	}

	return s.issueSession(ctx, user)
}

func (s *AuthService) generateJWT(user *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":            user.UserID.String(),
		"email":          user.Email,
		"is_protected":   user.IsProtected,
		"account_status": user.AccountStatus,
		"exp":            time.Now().Add(accessTokenTTL).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}
