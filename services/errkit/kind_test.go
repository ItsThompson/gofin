package errkit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/errkit"
)

// The Kind values are the error_kind tag vocabulary shared with the frontend
// helper and with every saved Sentry query, so their spellings are a wire
// contract rather than an implementation detail.
func TestKind_Vocabulary(t *testing.T) {
	tests := []struct {
		kind errkit.Kind
		want string
	}{
		{errkit.KindValidation, "validation"},
		{errkit.KindNotFound, "not_found"},
		{errkit.KindConflict, "conflict"},
		{errkit.KindPermission, "permission"},
		{errkit.KindUpstream, "upstream"},
		{errkit.KindTimeout, "timeout"},
		{errkit.KindDatabase, "database"},
		{errkit.KindInternal, "internal"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			assert.Equal(t, tc.want, string(tc.kind))
		})
	}
}
