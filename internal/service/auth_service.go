package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"

	"github.com/tuusuario/nomad-api/internal/config"
	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

var (
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrMinorNotSupportedYet = errors.New("support for users under 18 is coming soon")
	ErrInvalidBirthDate     = errors.New("birth_date must be in YYYY-MM-DD format")
)

type AuthService struct {
	userRepo *repo.UserRepository
	cfg      config.Config
}

func NewAuthService(ur *repo.UserRepository, cfg config.Config) *AuthService {
	return &AuthService{userRepo: ur, cfg: cfg}
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
	payload, err := idtoken.Validate(ctx, req.IDToken, s.cfg.GoogleCID)
	if err != nil {
		return nil, errors.New("invalid google token")
	}

	email := payload.Claims["email"].(string)

	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if err == repo.ErrUserNotFound {
			return nil, errors.New("user not registered, please register first")
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
