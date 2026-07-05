package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
)

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(us *service.UserService) *UserHandler {
	return &UserHandler{userService: us}
}

// PATCH /api/me/privacy
func (h *UserHandler) UpdatePrivacy(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	isProtected := c.MustGet("isProtected").(bool)

	var req domain.PrivacyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.userService.UpdatePrivacy(c.Request.Context(), userID, isProtected, req.PrivacyMode); err != nil {
		if err == service.ErrProtectedCannotBePublic {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"privacy_mode": req.PrivacyMode})
}

// GET /api/users/:username
func (h *UserHandler) GetProfile(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	username := c.Param("username")

	result, err := h.userService.GetProfile(c.Request.Context(), requesterID, username)
	if err != nil {
		switch err {
		case service.ErrProfilePrivate, service.ErrUserNotVisible:
			c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if result.Full != nil {
		c.JSON(http.StatusOK, result.Full)
	} else {
		c.JSON(http.StatusOK, result.Partial)
	}
}

// POST /api/users/:user_id/block
func (h *UserHandler) Block(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)

	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.userService.Block(c.Request.Context(), requesterID, targetID); err != nil {
		if err == service.ErrCannotBlockYourself {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user blocked"})
}

// DELETE /api/users/:user_id/block
func (h *UserHandler) Unblock(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)

	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.userService.Unblock(c.Request.Context(), requesterID, targetID); err != nil {
		if err == repo.ErrNotBlocked {
			c.JSON(http.StatusNotFound, gin.H{"error": "no block relationship found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "user unblocked"})
}

// DELETE /api/me
func (h *UserHandler) DeleteAccount(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	tokenExpiry := c.MustGet("tokenExpiry").(time.Time)

	if err := h.userService.DeleteAccount(c.Request.Context(), userID, tokenExpiry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}
