// Package serverkit owns the bootstrap spine and serve/shutdown lifecycle
// shared by all gofin Go services: JSON slog logger construction, Postgres
// connection (migrate + pool + ping), gin router and gRPC server assembly, and
// a single Serve function that runs the servers, blocks until the context is
// cancelled, performs a bounded graceful shutdown, and surfaces the first fatal
// serve error (e.g. a bind failure) instead of leaving a zombie process.
//
// serverkit takes primitive inputs only (level, dbURL, isProduction) so it has
// no dependency on any service's config package.
package serverkit
