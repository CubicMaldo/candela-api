package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/service"
)

type FollowHandler struct {
	followService *service.FollowService
}

func NewFollowHandler(fs *service.FollowService) *FollowHandler {
	return &FollowHandler{followService: fs}
}

// POST /api/places/:place_id/follow
func (h *FollowHandler) Follow(c *gin.Context) {
	placeID, err := uuid.Parse(c.Param("place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid place_id"})
		return
	}

	followerID := c.MustGet("userID").(uuid.UUID)

	followerCount, err := h.followService.Follow(c.Request.Context(), followerID, placeID)
	if err != nil {
		if err == service.ErrPlaceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"following": true, "follower_count": followerCount})
}

// DELETE /api/places/:place_id/follow
func (h *FollowHandler) Unfollow(c *gin.Context) {
	placeID, err := uuid.Parse(c.Param("place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid place_id"})
		return
	}

	followerID := c.MustGet("userID").(uuid.UUID)

	followerCount, err := h.followService.Unfollow(c.Request.Context(), followerID, placeID)
	if err != nil {
		if err == service.ErrNotFollowing {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"following": false, "follower_count": followerCount})
}
