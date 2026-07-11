// Package httpx provides the request guards duplicated across the GoFin HTTP
// handlers: trusted X-User-ID presence and JSON body binding. Both write an
// apierr response on failure so handlers collapse to a one-line guard.
package httpx

import (
	"github.com/gin-gonic/gin"

	"github.com/ItsThompson/gofin/services/apierr"
)

// RequireUserID reads the trusted X-User-ID header injected by the gateway
// after token validation. On an empty or missing header it writes a 401
// apierr.Error and returns ok=false; the caller must return immediately.
func RequireUserID(c *gin.Context) (userID string, ok bool) {
	userID = c.GetHeader("X-User-ID")
	if userID == "" {
		apierr.Respond(c, apierr.Unauthorized("Authentication required"))
		return "", false
	}
	return userID, true
}
