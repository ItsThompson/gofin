package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
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

// RegisterRoutes sets up the Gin routes for deletion endpoints.
func (h *DeletionHandler) RegisterRoutes(r *gin.Engine) {
	deletions := r.Group("/api/datarights/deletions")
	{
		deletions.POST("", h.CreateDeletion)
		deletions.GET("/:id", h.GetDeletion)
	}
}

// CreateDeletion handles POST /api/datarights/deletions.
// Creates a new deletion job and returns 202 Accepted.
func (h *DeletionHandler) CreateDeletion(c *gin.Context) {
	adminUserID := c.GetHeader("X-User-ID")
	if adminUserID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	var req model.CreateDeletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.ApiError{
			Code:    model.ErrValidationError,
			Message: "Invalid request body: userId and password are required",
		})
		return
	}

	job, err := h.deletionService.CreateJob(c.Request.Context(), req.UserID, adminUserID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.logger.Info("deletion job created via API",
		slog.String("job_id", job.ID),
		slog.String("user_id", job.UserID),
		slog.String("admin_user_id", adminUserID),
	)

	c.JSON(http.StatusAccepted, model.DeletionJobResponse{
		ID:          job.ID,
		UserID:      job.UserID,
		Status:      job.Status,
		Error:       job.Error,
		CreatedAt:   job.CreatedAt,
		CompletedAt: job.CompletedAt,
	})
}

// GetDeletion handles GET /api/datarights/deletions/:id.
// Returns the current state of a deletion job.
func (h *DeletionHandler) GetDeletion(c *gin.Context) {
	adminUserID := c.GetHeader("X-User-ID")
	if adminUserID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	jobID := c.Param("id")

	job, err := h.deletionService.GetJob(c.Request.Context(), jobID)
	if err != nil {
		h.handleError(c, err)
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

// handleError maps service errors to HTTP responses.
func (h *DeletionHandler) handleError(c *gin.Context, err error) {
	if svcErr, ok := err.(*service.ServiceError); ok {
		c.JSON(svcErr.Status, model.ApiError{
			Code:    svcErr.Code,
			Message: svcErr.Message,
		})
		return
	}

	h.logger.Error("unexpected error in deletion handler",
		slog.String("error", err.Error()),
	)
	c.JSON(http.StatusInternalServerError, model.ApiError{
		Code:    model.ErrInternalServerError,
		Message: "An unexpected error occurred",
	})
}
