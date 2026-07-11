package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/access"
	exportmetrics "github.com/ItsThompson/gofin/services/datarights/internal/metrics"
	"github.com/ItsThompson/gofin/services/datarights/internal/model"
	"github.com/ItsThompson/gofin/services/datarights/internal/service"
	"github.com/ItsThompson/gofin/services/httpx"
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

// RegisterRoutes registers every datarights-owned route from the shared access
// Registry, binding handlers from both the export and deletion handlers by ID.
// It is the single registration entry point shared by main.go and the
// registration coverage test. datarights is the one service whose routes span
// two handlers, so it merges both ID->handler maps before binding; a route can
// never be served without a Registry entry (which carries its access level).
func RegisterRoutes(r *gin.Engine, rest *RESTHandler, deletion *DeletionHandler) {
	handlers := make(map[string]gin.HandlerFunc)
	for id, fn := range rest.handlers() {
		handlers[id] = fn
	}
	for id, fn := range deletion.handlers() {
		handlers[id] = fn
	}

	access.BindRoutes("datarights", handlers, func(method, path string, handler gin.HandlerFunc) {
		r.Handle(method, path, handler)
	})
}

// handlers maps the datarights export Registry route IDs to gin handlers.
func (h *RESTHandler) handlers() map[string]gin.HandlerFunc {
	return map[string]gin.HandlerFunc{
		"datarights.exports.create": h.CreateExport,
		"datarights.exports.list":   h.ListExports,
		"datarights.exports.get":    h.GetExport,
	}
}

// CreateExport handles POST /api/datarights/exports.
// Returns 202 for new jobs, 200 for deduplicated in-progress jobs, or 429 for rate limited.
func (h *RESTHandler) CreateExport(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
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
		respondError(c, h.logger, err)
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
	userID, ok := httpx.RequireUserID(c)
	if !ok {
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
		respondError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetExport handles GET /api/datarights/exports/:id.
// Returns a single export job if owned by the authenticated user.
func (h *RESTHandler) GetExport(c *gin.Context) {
	userID, ok := httpx.RequireUserID(c)
	if !ok {
		return
	}

	jobID := c.Param("id")

	job, err := h.exportService.GetJob(c.Request.Context(), jobID, userID)
	if err != nil {
		respondError(c, h.logger, err)
		return
	}

	c.JSON(http.StatusOK, model.JobResponse{Job: job})
}
