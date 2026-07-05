package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/service"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(as *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: as}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		switch err {
		case service.ErrMinorNotSupportedYet, service.ErrInvalidBirthDate:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrEmailAlreadyUsed:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.AuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		if err == service.ErrInvalidCredentials || err == service.ErrGoogleOnlyAccount {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	var req domain.GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id_token is required"})
		return
	}

	resp, err := h.authService.GoogleLogin(c.Request.Context(), req)
	if err != nil {
		// Valid Google account without a Candela user: not an error for the
		// client — it continues to the complete-profile step.
		var needsReg *service.GoogleNeedsRegistrationError
		if errors.As(err, &needsReg) {
			c.JSON(http.StatusOK, domain.GoogleNeedsRegistration{
				NeedsRegistration: true,
				Email:             needsReg.Email,
			})
			return
		}
		if err == service.ErrGoogleTokenInvalid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) GoogleRegister(c *gin.Context) {
	var req domain.GoogleRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.GoogleRegister(c.Request.Context(), req)
	if err != nil {
		switch err {
		case service.ErrMinorNotSupportedYet, service.ErrInvalidBirthDate:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrEmailAlreadyUsed:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case service.ErrGoogleTokenInvalid:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}
