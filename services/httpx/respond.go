package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/access"
	"github.com/ItsThompson/gofin/services/apierr"
	"github.com/ItsThompson/gofin/services/errkit"
)

// RespondError writes err's HTTP error response and reports the failure when err
// is not a classified *apierr.Error. It is the one wrapper the REST handlers share:
// apierr.Respond owns the wire mapping and must stay free of the Sentry SDK, while
// errkit must stay free of gin, so the place that holds both concerns is here.
//
// meta carries the calling handler's domain. An empty meta.Op is filled with the
// Registry ID of the route being served, e.g. "expense.create", because those IDs
// are already the bounded per-route names the operation tag and the group key
// want. A route the Registry does not declare leaves it empty, and the group key
// falls back to the kind.
//
// A classified error is never reported: an *apierr.Error carries an explicit Status
// and every 4xx in the codebase is one, so the whole client-error class is excluded
// by construction rather than by an enumerated list of statuses. The report reads
// the hub from the request context, where sentrygin put it; a background context
// would fall back to a clone of the global hub and lose the request.
func RespondError(c *gin.Context, err error, meta errkit.Meta) {
	var apiErr *apierr.Error
	if !errors.As(err, &apiErr) {
		if meta.Op == "" {
			meta.Op = access.RouteID(c.Request.Method, c.FullPath())
		}
		_ = errkit.Report(c.Request.Context(), err, meta)
	}
	apierr.Respond(c, err)
}
