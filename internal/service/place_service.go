package service

import (
	"context"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/repo"
)

type PlaceService struct {
	placeRepo *repo.PlaceRepository
}

func NewPlaceService(pr *repo.PlaceRepository) *PlaceService {
	return &PlaceService{placeRepo: pr}
}

func (s *PlaceService) SearchNearby(ctx context.Context, req domain.SearchPlacesRequest) ([]domain.Place, error) {
	// PostGIS evalúa las distancias geográficas en metros
	radiusMeters := req.RadiusKM * 1000.0

	// Delegamos la validación espacial al motor robusto de PostgreSQL
	return s.placeRepo.FindNearby(ctx, req.Lat, req.Lon, radiusMeters, req.Category)
}
