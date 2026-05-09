package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	exportmetrics "github.com/ItsThompson/gofin/services/datarights/internal/metrics"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
)

// RESTHandler handles HTTP requests for the datarights service.
type RESTHandler struct {
	exportService *service.ExportService
	logger        *slog.Logger
}

// NewRESTHandler creates a new RESTHandler.
func NewRESTHandler(exportService *service.ExportService, logger *slog.Logger) *RESTHandler {
	return &RESTHandler{
		exportService: exportService,
		logger:        logger,
	}
}

// RegisterRoutes sets up the Gin routes for datarights endpoints.
func (h *RESTHandler) RegisterRoutes(r *gin.Engine) {
	exports := r.Group("/api/datarights/exports")
	{
		exports.POST("", h.CreateExport)
		exports.GET("", h.ListExports)
		exports.GET("/:id", h.GetExport)
	}
}

// CreateExport handles POST /api/datarights/exports.
// Returns 202 for new jobs, 200 for deduplicated in-progress jobs, or 429 for rate limited.
func (h *RESTHandler) CreateExport(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	result, err := h.exportService.CreateJob(c.Request.Context(), userID)
	if err != nil {
		if rateLimitErr, ok := err.(*service.RateLimitError); ok {
			exportmetrics.ExportRateLimitRejectionsTotal.Inc()
			h.logger.Warn("export rate limit rejected",
				slog.String("user_id", userID),
				slog.Time("next_allowed_at", rateLimitErr.RetryAfter),
				slog.String("method", "handler.CreateExport"),
			)
			c.JSON(http.StatusTooManyRequests, model.RateLimitedResponse{
				Code:       model.ErrRateLimited,
				Message:    "Export limit reached. You can request another export after " + rateLimitErr.RetryAfter.Format("2006-01-02") + ".",
				RetryAfter: rateLimitErr.RetryAfter,
			})
			return
		}
		h.handleError(c, err)
		return
	}

	if result.IsExisting {
		c.JSON(http.StatusOK, model.JobResponse{Job: result.Job})
		return
	}

	exportmetrics.ExportJobsCreatedTotal.Inc()
	h.logger.Info("export job created",
		slog.String("job_id", result.Job.ID),
		slog.String("user_id", userID),
		slog.String("method", "handler.CreateExport"),
	)
	c.JSON(http.StatusAccepted, model.JobResponse{Job: result.Job})
}

// ListExports handles GET /api/datarights/exports.
// Returns a paginated list of the user's export jobs.
func (h *RESTHandler) ListExports(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	page := 1
	pageSize := 10

	if p := c.Query("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	if ps := c.Query("pageSize"); ps != "" {
		if parsed, err := strconv.Atoi(ps); err == nil && parsed > 0 {
			pageSize = parsed
		}
	}

	result, err := h.exportService.ListJobs(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetExport handles GET /api/datarights/exports/:id.
// Returns a single export job if owned by the authenticated user.
func (h *RESTHandler) GetExport(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, model.ApiError{
			Code:    model.ErrUnauthorized,
			Message: "Authentication required",
		})
		return
	}

	jobID := c.Param("id")

	job, err := h.exportService.GetJob(c.Request.Context(), jobID, userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, model.JobResponse{Job: job})
}

// handleError maps service errors to HTTP responses.
func (h *RESTHandler) handleError(c *gin.Context, err error) {
	if svcErr, ok := err.(*service.ServiceError); ok {
		c.JSON(svcErr.Status, model.ApiError{
			Code:    svcErr.Code,
			Message: svcErr.Message,
		})
		return
	}

	h.logger.Error("unexpected error",
		slog.String("error", err.Error()),
	)
	c.JSON(http.StatusInternalServerError, model.ApiError{
		Code:    model.ErrInternalServerError,
		Message: "An unexpected error occurred",
	})
}
