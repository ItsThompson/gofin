package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/errkit"
	"github.com/ItsThompson/gofin/services/httpx"
)

// respondError reports an unclassified failure and writes the shared error
// response. The operation comes from the route, so only the domain is per service.
// The report writes the log record, so no caller passes a logger.
func respondError(c *gin.Context, err error) {
	httpx.RespondError(c, err, errkit.Meta{
		Domain: "datarights",
		Msg:    "unexpected error",
	})
}
