package handler

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/apierr"
)

// respondError maps a service error to an HTTP response via apierr.Respond.
// apierr.Respond classifies an *apierr.Error to its status/code and everything
// else to a 500; because it takes no logger, this helper logs unclassified
// errors first so the unexpected-500 observability the old handleError provided
// is preserved.
func respondError(c *gin.Context, logger *slog.Logger, err error) {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		logger.Error("unexpected error", slog.String("error", err.Error()))
	}
	apierr.Respond(c, err)
}
