package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/repo"
	"github.com/tuusuario/candela-api/internal/service"
)

type FriendHandler struct {
	friendService *service.FriendService
}

func NewFriendHandler(fs *service.FriendService) *FriendHandler {
	return &FriendHandler{friendService: fs}
}

// POST /api/users/:user_id/friend-request
func (h *FriendHandler) SendRequest(c *gin.Context) {
	senderID := c.MustGet("userID").(uuid.UUID)
	targetID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.friendService.SendRequest(c.Request.Context(), senderID, targetID); err != nil {
		switch err {
		case service.ErrCannotFriendYourself:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrUserNotVisible:
			c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado."})
		case service.ErrFriendRequestAlreadyExists, service.ErrAlreadyFriends:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "friend request sent"})
}

// POST /api/friend-requests/:user_id/accept
func (h *FriendHandler) AcceptRequest(c *gin.Context) {
	recipientID := c.MustGet("userID").(uuid.UUID)
	senderID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.friendService.AcceptRequest(c.Request.Context(), recipientID, senderID); err != nil {
		switch err {
		case repo.ErrFriendRequestNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "friend request not found"})
		case service.ErrNotRecipient:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "friend request accepted"})
}

// POST /api/friend-requests/:user_id/reject
func (h *FriendHandler) RejectRequest(c *gin.Context) {
	recipientID := c.MustGet("userID").(uuid.UUID)
	senderID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.friendService.RejectRequest(c.Request.Context(), recipientID, senderID); err != nil {
		switch err {
		case repo.ErrFriendRequestNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "friend request not found"})
		case service.ErrNotRecipient:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// DELETE /api/friend-requests/:user_id
func (h *FriendHandler) CancelRequest(c *gin.Context) {
	senderID := c.MustGet("userID").(uuid.UUID)
	recipientID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.friendService.CancelRequest(c.Request.Context(), senderID, recipientID); err != nil {
		switch err {
		case repo.ErrFriendRequestNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "friend request not found"})
		case service.ErrNotRequester:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// DELETE /api/users/:user_id/friend
func (h *FriendHandler) RemoveFriend(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	friendID, err := uuid.Parse(c.Param("user_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.friendService.RemoveFriend(c.Request.Context(), userID, friendID); err != nil {
		if err == repo.ErrNotFriends {
			c.JSON(http.StatusNotFound, gin.H{"error": "not friends"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// GET /api/friend-requests/incoming
func (h *FriendHandler) GetIncoming(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	entries, err := h.friendService.GetIncomingRequests(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []domain.FriendRequestEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"requests": entries, "total": len(entries)})
}

// GET /api/friend-requests/outgoing
func (h *FriendHandler) GetOutgoing(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	entries, err := h.friendService.GetOutgoingRequests(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []domain.FriendRequestEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"requests": entries, "total": len(entries)})
}

// GET /api/me/friends
func (h *FriendHandler) GetFriends(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	entries, err := h.friendService.GetFriends(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []domain.FriendEntry{}
	}
	c.JSON(http.StatusOK, gin.H{"friends": entries, "total": len(entries)})
}
