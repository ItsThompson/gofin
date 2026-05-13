// Package migrations provides embedded SQL migration files for the auth service.
// Using go:embed makes the binary self-contained: no need to COPY migration files
// into the container at runtime, enabling distroless/scratch base images.
package migrations

import "embed"

// FS contains all SQL migration files for the auth service database schema.
//
//go:embed *.sql
var FS embed.FS
