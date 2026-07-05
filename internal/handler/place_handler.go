package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/service"
)

type PlaceHandler struct {
	placeService *service.PlaceService
}

func NewPlaceHandler(ps *service.PlaceService) *PlaceHandler {
	return &PlaceHandler{placeService: ps}
}

func (h *PlaceHandler) SearchNearby(c *gin.Context) {
	var req domain.SearchPlacesRequest

	// Extraemos los query parameters (?lat=X&lon=Y&radius=Z)
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	places, err := h.placeService.SearchNearby(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch nearby places"})
		return
	}

	// Devolver arreglo vacío en lugar de null si no hay resultados en esa zona
	if places == nil {
		places = []domain.Place{}
	}

	c.JSON(http.StatusOK, gin.H{"data": places})
}

// GET /api/places/:place_id
func (h *PlaceHandler) GetDetail(c *gin.Context) {
	placeID, err := uuid.Parse(c.Param("place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid place_id"})
		return
	}

	requesterID := c.MustGet("userID").(uuid.UUID)

	detail, err := h.placeService.GetDetail(c.Request.Context(), requesterID, placeID)
	if err != nil {
		if err == service.ErrPlaceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// GET /api/places/:place_id/feed
func (h *PlaceHandler) GetEventFeed(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	placeID, err := uuid.Parse(c.Param("place_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid place_id"})
		return
	}

	limit := parseLimitParam(c.Query("limit"), 20, 50)
	before := parseBeforeParam(c.Query("before"))

	feed, err := h.placeService.GetEventFeed(c.Request.Context(), requesterID, placeID, limit, before)
	if err != nil {
		if err == service.ErrPlaceNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, feed)
}
