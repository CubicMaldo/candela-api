package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/service"
)

type CheckInHandler struct {
	checkinService *service.CheckInService
}

func NewCheckInHandler(cs *service.CheckInService) *CheckInHandler {
	return &CheckInHandler{checkinService: cs}
}

// POST /api/places/:place_id/checkin/initiate
func (h *CheckInHandler) Initiate(c *gin.Context) {
	placeID, err := uuid.Parse(c.Param("place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid place_id"})
		return
	}

	userID := c.MustGet("userID").(uuid.UUID)
	isProtected := c.MustGet("isProtected").(bool)

	var req domain.CheckInInitiateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.checkinService.Initiate(c.Request.Context(), userID, placeID, isProtected, req)
	if err != nil {
		switch err {
		case service.ErrNotWithinGeofence:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrAlreadyCheckedIn:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case service.ErrPlaceRestricted:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

// POST /api/places/:place_id/checkin/confirm
func (h *CheckInHandler) Confirm(c *gin.Context) {
	placeID, err := uuid.Parse(c.Param("place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid place_id"})
		return
	}

	userID := c.MustGet("userID").(uuid.UUID)

	var req domain.CheckInConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.checkinService.Confirm(c.Request.Context(), userID, placeID, req)
	if err != nil {
		switch err {
		case service.ErrInvalidCheckInNonce:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrAlreadyCheckedIn:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, result)
}

// PATCH /api/checkins/:checkin_id/permanent
func (h *CheckInHandler) SetPermanent(c *gin.Context) {
	checkinID, err := uuid.Parse(c.Param("checkin_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid checkin_id"})
		return
	}

	userID := c.MustGet("userID").(uuid.UUID)

	var req domain.PermanentUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	checkin, err := h.checkinService.SetPermanent(c.Request.Context(), userID, checkinID, req.Permanent)
	if err != nil {
		switch err {
		case service.ErrCheckInNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case service.ErrNotCheckInOwner:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrEventClosed:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"checkin_id":         checkin.CheckInID,
		"photo_is_permanent": checkin.PhotoIsPermanent,
	})
}
