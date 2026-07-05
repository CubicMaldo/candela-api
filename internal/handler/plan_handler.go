package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/candela-api/internal/domain"
	"github.com/tuusuario/candela-api/internal/service"
)

type PlanHandler struct {
	planService *service.PlanService
}

func NewPlanHandler(ps *service.PlanService) *PlanHandler {
	return &PlanHandler{planService: ps}
}

// POST /api/plans
func (h *PlanHandler) Create(c *gin.Context) {
	creatorID := c.MustGet("userID").(uuid.UUID)

	var req domain.CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plan, err := h.planService.Create(c.Request.Context(), creatorID, req)
	if err != nil {
		switch err {
		case service.ErrCannotPlanWithSelf, service.ErrInvalidDeadlineHours,
			service.ErrTargetNotFriend, service.ErrPlaceNotActive,
			service.ErrTargetRequired, service.ErrTargetNotAllowed:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrUserNotVisible:
			c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado."})
		case service.ErrDuplicatePlan, service.ErrDuplicateOpenPlan:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, plan)
}

// GET /api/plans/discover?lat=&lon=&radius_km=
func (h *PlanHandler) Discover(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)

	var req domain.DiscoverPlansRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	plans, err := h.planService.Discover(
		c.Request.Context(), requesterID, req.Lat, req.Lon, req.RadiusKM*1000.0,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans, "total": len(plans)})
}

// POST /api/plans/:plan_id/join
func (h *PlanHandler) Join(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	planID, err := uuid.Parse(c.Param("plan_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan_id"})
		return
	}

	plan, err := h.planService.Join(c.Request.Context(), requesterID, planID)
	if err != nil {
		switch err {
		case service.ErrPlanNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan no encontrado."})
		case service.ErrPlanNotJoinable, service.ErrCannotJoinOwnPlan:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrPlanNotPending, service.ErrPlanExpired:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrAlreadyInPlan:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, plan)
}

// POST /api/plans/:plan_id/accept
func (h *PlanHandler) Accept(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	planID, err := uuid.Parse(c.Param("plan_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan_id"})
		return
	}

	detail, err := h.planService.Accept(c.Request.Context(), requesterID, planID)
	if err != nil {
		switch err {
		case service.ErrPlanNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan no encontrado."})
		case service.ErrNotPlanTarget:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrPlanNotPending, service.ErrPlanExpired:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, detail)
}

// POST /api/plans/:plan_id/decline
func (h *PlanHandler) Decline(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	planID, err := uuid.Parse(c.Param("plan_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan_id"})
		return
	}

	if err := h.planService.Decline(c.Request.Context(), requesterID, planID); err != nil {
		switch err {
		case service.ErrPlanNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan no encontrado."})
		case service.ErrNotPlanTarget:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrPlanNotPending:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// DELETE /api/plans/:plan_id
func (h *PlanHandler) Cancel(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	planID, err := uuid.Parse(c.Param("plan_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan_id"})
		return
	}

	if err := h.planService.Cancel(c.Request.Context(), requesterID, planID); err != nil {
		switch err {
		case service.ErrPlanNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan no encontrado."})
		case service.ErrNotPlanCreator:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case service.ErrPlanNotPending:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

// GET /api/plans/incoming
func (h *PlanHandler) GetIncoming(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	results, err := h.planService.GetIncoming(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": results, "total": len(results)})
}

// GET /api/plans/outgoing
func (h *PlanHandler) GetOutgoing(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)
	results, err := h.planService.GetOutgoing(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": results, "total": len(results)})
}

// GET /api/plans/:plan_id
func (h *PlanHandler) GetByID(c *gin.Context) {
	requesterID := c.MustGet("userID").(uuid.UUID)
	planID, err := uuid.Parse(c.Param("plan_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan_id"})
		return
	}

	detail, err := h.planService.GetByID(c.Request.Context(), requesterID, planID)
	if err != nil {
		switch err {
		case service.ErrPlanNotFound, service.ErrNotPlanParticipant:
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan no encontrado."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, detail)
}
