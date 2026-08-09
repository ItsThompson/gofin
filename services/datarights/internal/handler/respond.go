package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/httpx"
)

// respondError maps a service error to an HTTP response via apierr.Respond and
// reports an unclassified failure through the shared httpx helper, which names the
// operation from this route's Registry ID. The report writes the log record, so
// the caller supplies no logger.
func respondError(c *gin.Context, err error) {
	httpx.RespondError(c, err, errkit.Meta{
		Domain: "datarights",
		Msg:    "unexpected error",
	})
}
