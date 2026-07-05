package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/service"
)

type RankingHandler struct {
	rankingService *service.RankingService
}

func NewRankingHandler(rs *service.RankingService) *RankingHandler {
	return &RankingHandler{rankingService: rs}
}

// PATCH /api/me/ranking
func (h *RankingHandler) UpdateOptIn(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var req domain.RankingOptInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.rankingService.UpdateOptIn(c.Request.Context(), userID, *req.RankingOptIn); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ranking_opt_in": *req.RankingOptIn})
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
