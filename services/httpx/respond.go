package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
)

// RespondError writes err's HTTP error response and reports the failure when err
// is not a classified *apierr.Error. It replaces the near-identical wrapper each
// REST handler carried, which logged an unclassified error and then delegated the
// wire mapping, because apierr.Respond takes no logger and must stay free of the
// Sentry SDK.
//
// meta carries the operation and domain of the calling handler. The wire response
// is apierr.Respond's alone, so reporting here changes no response body.
//
// A classified error is not reported: an *apierr.Error carries an explicit Status
// and every 4xx in the codebase is one, so the whole client-error class is
// excluded by construction rather than by an enumerated list of statuses.
//
// The report reads the hub from the request context, which is where sentrygin put
// it, so the event carries the request. A background context would fall back to a
// clone of the global hub and lose it.
func RespondError(c *gin.Context, err error, meta errkit.Meta) {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		_ = errkit.Report(c.Request.Context(), err, meta)
	}
	apierr.Respond(c, err)
}
