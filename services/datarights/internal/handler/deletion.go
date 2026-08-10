package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	"github.com/ItsThompson/gofin/services/httpx"
)

// DeletionHandler handles HTTP requests for deletion job endpoints.
type DeletionHandler struct {
	deletionService *service.DeletionService
	logger          *slog.Logger
}

// NewDeletionHandler creates a new DeletionHandler.
func NewDeletionHandler(deletionService *service.DeletionService, logger *slog.Logger) *DeletionHandler {
	return &DeletionHandler{
		deletionService: deletionService,
		logger:          logger,
	}
}

// handlers maps the datarights deletion Registry route IDs to gin handlers.
func (h *DeletionHandler) handlers() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		"datarights.deletions.create": h.CreateDeletion,
		"datarights.deletions.get":    h.GetDeletion,
	}
}

// CreateDeletion handles POST /api/datarights/deletions.
// Returns 202 Accepted for a new job, or 200 OK for an existing in-progress job.
func (h *DeletionHandler) CreateDeletion(c *gin.Context) {
	adminUserID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	var req model.CreateDeletionRequest
	if !httpx.BindJSON(c, &req) {
		return
	}

	result, err := h.deletionService.CreateJob(c.Request.Context(), req.UserID, adminUserID, req.Password)
	if err != nil {
		respondError(c, err)
		return
	}

	job := result.Job
	resp := model.DeletionJobResponse{
		ID:          job.ID,
		UserID:      job.UserID,
		Status:      job.Status,
		Error:       job.Error,
		CreatedAt:   job.CreatedAt,
		CompletedAt: job.CompletedAt,
	}

	if result.IsExisting {
		c.JSON(http.StatusOK, resp)
	} else {
		h.logger.Info("deletion job created via API",
			slog.String("job_id", job.ID),
			slog.String("user_id", job.UserID),
			slog.String("admin_user_id", adminUserID),
		)
		c.JSON(http.StatusAccepted, resp)
	}
}

// GetDeletion handles GET /api/datarights/deletions/:id.
// Returns the current state of a deletion job.
func (h *DeletionHandler) GetDeletion(c *gin.Context) {
	if _, ok := httpx.RequireUserID(c); !ok {
		return
	}

	jobID := c.Param("id")

	job, err := h.deletionService.GetJob(c.Request.Context(), jobID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.DeletionJobResponse{
		ID:          job.ID,
		UserID:      job.UserID,
		Status:      job.Status,
		Error:       job.Error,
		CreatedAt:   job.CreatedAt,
		CompletedAt: job.CompletedAt,
	})
}
