package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/service"
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
