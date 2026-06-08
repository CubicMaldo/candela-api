package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/tuusuario/nomad-api/internal/domain"
	"github.com/tuusuario/nomad-api/internal/service"
)

type ReportHandler struct {
	reportService *service.ReportService
}

func NewReportHandler(rs *service.ReportService) *ReportHandler {
	return &ReportHandler{reportService: rs}
}

// POST /api/reports
func (h *ReportHandler) Create(c *gin.Context) {
	reporterID := c.MustGet("userID").(uuid.UUID)

	var req domain.CreateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := h.reportService.CreateReport(c.Request.Context(), reporterID, req)
	if err != nil {
		switch err {
		case service.ErrCannotReportSelf:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case service.ErrReportTargetNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"report_id": report.ID,
		"status":    report.Status,
	})
}
