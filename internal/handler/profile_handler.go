package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/repo"
	"github.com/tuusuario/nomad-api/internal/service"
)

type ProfileHandler struct {
	profileService *service.ProfileService
}

func NewProfileHandler(ps *service.ProfileService) *ProfileHandler {
	return &ProfileHandler{profileService: ps}
}

// GET /api/me
func (h *ProfileHandler) GetMe(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	profile, err := h.profileService.GetMyProfile(c.Request.Context(), userID)
	if err != nil {
		if err == repo.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Account not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// GET /api/feed
func (h *ProfileHandler) GetFeed(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	limit := parseLimitParam(c.Query("limit"), 20, 50)
	before := parseBeforeParam(c.Query("before"))

	events, cursor, err := h.profileService.GetFeed(c.Request.Context(), userID, limit, before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var nextCursor interface{}
	if cursor != nil {
		nextCursor = cursor.UTC().Format(time.RFC3339Nano)
	}

	c.JSON(http.StatusOK, gin.H{
		"events":      events,
		"next_cursor": nextCursor,
	})
}

// GET /api/me/check-ins
func (h *ProfileHandler) GetMyCheckIns(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	limit := parseLimitParam(c.Query("limit"), 20, 50)
	before := parseBeforeParam(c.Query("before"))

	results, cursor, err := h.profileService.GetMyCheckIns(c.Request.Context(), userID, limit, before)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var nextCursor interface{}
	if cursor != nil {
		nextCursor = cursor.UTC().Format(time.RFC3339Nano)
	}

	c.JSON(http.StatusOK, gin.H{
		"check_ins":   results,
		"total":       len(results),
		"next_cursor": nextCursor,
	})
}

// GET /api/me/plans
func (h *ProfileHandler) GetMyPlans(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	statusFilter := c.Query("status") // "active" | "completed" | "failed" | ""

	results, err := h.profileService.GetMyPlans(c.Request.Context(), userID, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plans": results,
		"total": len(results),
	})
}

// parseLimitParam parses a limit query param, clamping between 1 and max.
func parseLimitParam(raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// parseBeforeParam parses an RFC3339 ISO timestamp cursor param.
func parseBeforeParam(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return nil
		}
	}
	return &t
}
