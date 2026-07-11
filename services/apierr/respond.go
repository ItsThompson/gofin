package apierr

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError is the JSON wire shape for every error response: {code, message,
// fields?}. It is byte-identical to what the services emit today; only the
// exported name adopts the APIError initialism (US-HARDEN-04). JSON tags are
// unchanged.
type APIError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// Respond maps any error to a gin JSON response following {code, message,
// fields?}. It classifies via errors.As so a %w-wrapped *Error is still mapped
// to its own status/code. Unknown errors fall through to 500 CodeInternal.
func Respond(c *gin.Context, err error) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		c.JSON(apiErr.Status, APIError{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Fields:  apiErr.Fields,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, APIError{
		Code:    CodeInternal,
		Message: "An unexpected error occurred",
	})
}
