package access_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ItsThompson/gofin/services/gateway/internal/access"
)

// BenchmarkValidateHappyPath measures the allocations on the validation happy
// path: an Authenticated route with a valid cookie whose token the validator
// accepts. It is the committed reference for services/gateway/perf/baseline.
//
// The absolute allocs/op is a same-machine reference, not a CI gate (see the
// services/perf README): this path's regression guard is behavioral (the
// bounded-timeout test), not an allocation count. The benchmark exists so the
// baseline file records a concrete "before" number alongside the note that the
// pre-change ValidateToken call had no upper latency bound.
func BenchmarkValidateHappyPath(b *testing.B) {
	validator := &fakeValidator{
		result: &access.TokenValidationResult{UserID: "user-1", Role: "user"},
	}
	engine := buildEngine(validator, silentLogger(), http.MethodPost, "/api/auth/restore", okHandler)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/restore", nil)
		req.AddCookie(&http.Cookie{Name: "gofin_access", Value: "token"})
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("unexpected status %d", rec.Code)
		}
	}
}
