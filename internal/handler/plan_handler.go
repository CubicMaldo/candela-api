package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/service"
)

type ChallengeHandler struct {
	challengeService *service.ChallengeService
}

func NewChallengeHandler(cs *service.ChallengeService) *ChallengeHandler {
	return &ChallengeHandler{challengeService: cs}
}

// POST /api/challenges
func (h *ChallengeHandler) Create(c *gin.Context) {
	creatorID := c.MustGet("userID").(uuid.UUID)

	var req domain.CreateChallengeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	challenge, err := h.challengeService.Create(c.Request.Context(), creatorID, req)
	if err != nil {
		switch err {
		case service.ErrCannotChallengeSelf, service.ErrInvalidDeadlineHours,
			service.ErrTargetNotFriend, service.ErrPlaceNotActive:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrUserNotVisible:
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case service.ErrDuplicateChallenge:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, challenge)
}

// POST /api/challenges/:challenge_id/accept
func (h *ChallengeHandler) Accept(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	challengeID, err := uuid.Parse(c.Param("challenge_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge_id"})
		return
	}

	detail, err := h.challengeService.Accept(c.Request.Context(), requesterID, challengeID)
	if err != nil {
		switch err {
		case service.ErrChallengeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "challenge not found"})
		case service.ErrNotChallengeTarget:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrChallengeNotPending, service.ErrChallengeExpired:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, detail)
}

// POST /api/challenges/:challenge_id/reject
func (h *ChallengeHandler) Reject(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	challengeID, err := uuid.Parse(c.Param("challenge_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge_id"})
		return
	}

	if err := h.challengeService.Reject(c.Request.Context(), requesterID, challengeID); err != nil {
		switch err {
		case service.ErrChallengeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "challenge not found"})
		case service.ErrNotChallengeTarget:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrChallengeNotPending:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GET /api/challenges/incoming
func (h *ChallengeHandler) GetIncoming(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	results, err := h.challengeService.GetIncoming(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"challenges": results, "total": len(results)})
}

// GET /api/challenges/outgoing
func (h *ChallengeHandler) GetOutgoing(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	results, err := h.challengeService.GetOutgoing(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"challenges": results, "total": len(results)})
}

// GET /api/challenges/:challenge_id
func (h *ChallengeHandler) GetByID(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	challengeID, err := uuid.Parse(c.Param("challenge_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge_id"})
		return
	}

	detail, err := h.challengeService.GetByID(c.Request.Context(), requesterID, challengeID)
	if err != nil {
		switch err {
		case service.ErrChallengeNotFound, service.ErrNotChallengeParticipant:
			c.JSON(http.StatusNotFound, gin.H{"error": "challenge not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, detail)
}
