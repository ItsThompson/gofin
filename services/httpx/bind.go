package httpx

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/ItsThompson/gofin/services/apierr"
)

// BindJSON binds the request body into dst. On failure it writes a 400
// apierr.Error, attaching field-level detail when the validator reports it,
// and returns false. It is generic over the target type so call sites stay a
// single line.
func BindJSON[T any](c *gin.Context, dst *T) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		apierr.Respond(c, apierr.Validation("Invalid request body", validationFields(err)))
		return false
	}
	return true
}

// validationFields extracts a field->failed-rule map from a binding error when
// it is a validator.ValidationErrors. Malformed-JSON and other bind errors
// carry no field detail, so it returns nil (Fields is omitted on the wire).
func validationFields(err error) map[string]string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return nil
	}

	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		fields[fe.Field()] = fe.Tag()
	}
	return fields
}
