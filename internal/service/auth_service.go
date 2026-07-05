package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
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

// googleTokenValidator verifies a Google ID token against the audience and
// returns the verified email. Injectable so tests can fake Google.
type googleTokenValidator func(ctx context.Context, idToken, audience string) (string, error)

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

// authUserRepo is the slice of UserRepository the auth service needs.
// *repo.UserRepository satisfies it; tests can substitute a fake.
type authUserRepo interface {
	Create(ctx context.Context, u *domain.User) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
}

type AuthService struct {
	userRepo        authUserRepo
	cfg             config.Config
	googleValidator googleTokenValidator
}

func NewAuthService(ur *repo.UserRepository, cfg config.Config) *AuthService {
	return &AuthService{userRepo: ur, cfg: cfg, googleValidator: defaultGoogleValidator}
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

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
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

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
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

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
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

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) generateJWT(user *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":            user.UserID.String(),
		"email":          user.Email,
		"is_protected":   user.IsProtected,
		"account_status": user.AccountStatus,
		"exp":            time.Now().Add(time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}
