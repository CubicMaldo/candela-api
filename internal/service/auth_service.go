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
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnderage           = errors.New("must be at least 18 years old")
)

type AuthService struct {
	userRepo *repo.UserRepository
	cfg      config.Config
}

func NewAuthService(ur *repo.UserRepository, cfg config.Config) *AuthService {
	return &AuthService{
		userRepo: ur,
		cfg:      cfg,
	}
}

func calculateAge(birthDate time.Time) int {
	now := time.Now()
	age := now.Year() - birthDate.Year()
	if now.YearDay() < birthDate.YearDay() {
		age--
	}
	return age
}

func (s *AuthService) Register(ctx context.Context, req domain.RegisterRequest) (*domain.AuthResponse, error) {
	age := calculateAge(req.BirthDate)
	if age < 18 {
		return nil, ErrUnderage
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:         req.Email,
		Username:      req.Username,
		PasswordHash:  string(hash),
		BirthDate:     req.BirthDate,
		Age:           age,
		AccountStatus: "ACTIVE", // For 18+, auto active
		PrivacyMode:   "FRIENDS", // default mode
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		// En la vida real, parsear el error para determinar si es duplicidad de keys
		return nil, err
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{
		Token: token,
		User:  *user,
	}, nil
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

	return &domain.AuthResponse{
		Token: token,
		User:  *user,
	}, nil
}

func (s *AuthService) GoogleLogin(ctx context.Context, req domain.GoogleAuthRequest) (*domain.AuthResponse, error) {
	// Validate token with Google
	payload, err := idtoken.Validate(ctx, req.IDToken, s.cfg.GoogleCID)
	if err != nil {
		return nil, errors.New("invalid google token")
	}

	email := payload.Claims["email"].(string)
	
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if err == repo.ErrUserNotFound {
			// Create user implicitly from google
			// Since we don't have birthdate from basic google scope, we might need a 2-step process, 
			// but for this MVP we can bypass age if logged in via Google if you allow it, or set default.
			// Ideally, we'd require the user to complete their profile with DOB.
			// We'll set a placeholder or return error requiring DOB.
			return nil, errors.New("user not registered, please register first")
		}
		return nil, err
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, err
	}

	return &domain.AuthResponse{
		Token: token,
		User:  *user,
	}, nil
}

func (s *AuthService) generateJWT(user *domain.User) (string, error) {
	claims := jwt.MapClaims{
		"sub": user.UserID.String(),
		"email": user.Email,
		"exp": time.Now().Add(72 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}
