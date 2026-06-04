package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/service"
)

type RankingHandler struct {
	rankingService *service.RankingService
}

func NewRankingHandler(rs *service.RankingService) *RankingHandler {
	return &RankingHandler{rankingService: rs}
}

// GET /api/rankings/global
func (h *RankingHandler) Global(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	resp, err := h.rankingService.GetGlobal(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /api/rankings/friends
func (h *RankingHandler) Friends(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	resp, err := h.rankingService.GetFriends(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
