package httpx

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const noStore = "no-store"

// NoStore marks a personal response as private to the current request. It is
// applied before the wrapped handler so success and error responses share the
// same policy.
func NoStore(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", noStore)
		handler(c)
	}
}

// IsCacheBypass reports whether Cache-Control contains the no-cache directive.
// Directive matching is case-insensitive and ignores optional parameters.
func IsCacheBypass(request *http.Request) bool {
	for _, headerValue := range request.Header.Values("Cache-Control") {
		for _, directive := range strings.Split(headerValue, ",") {
			name := strings.TrimSpace(strings.SplitN(directive, "=", 2)[0])
			if strings.EqualFold(name, "no-cache") {
				return true
			}
		}
	}
	return false
}
