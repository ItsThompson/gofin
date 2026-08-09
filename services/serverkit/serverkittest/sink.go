// Package serverkittest provides the log sink the panic-recovery tests share,
// exported so every module that installs a serverkit recovery reads the
// recovered-panic record through one seam instead of re-deriving the JSON
// parsing per package.
//
// A sink rather than an assertion helper: the record's attributes differ per
// site (a request path, a job id, a downstream name), and the frame each stack
// must reach is the whole point of the assertion, so those belong in the test
// that knows them. Only the plumbing is shared.
//
// It takes no *testing.T, so nothing here drags the testing package into a
// module's non-test build graph.
package serverkittest

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"sync"
)

// Sink is a mutex-guarded slog destination. A recovery normally writes its
// record from a different goroutine than the one asserting on it: a request
// handler, a worker goroutine, or a probe.
type Sink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// NewLogger returns a JSON slog logger at debug level writing to a fresh Sink.
// Debug level so a test can assert that a record was written *below* error
// level, which is how the aborted-connection path is checked.
func NewLogger() (*slog.Logger, *Sink) {
	sink := &Sink{}
	handler := slog.NewJSONHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), sink
}

func (s *Sink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

// Records returns every record written so far, in order.
func (s *Sink) Records() ([]map[string]any, error) {
	s.mu.Lock()
	raw := append([]byte(nil), s.buf.Bytes()...)
	s.mu.Unlock()

	var records []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for decoder.More() {
		var record map[string]any
		if err := decoder.Decode(&record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

// RecordsAtLevel returns the records written at one slog level ("ERROR",
// "WARN", ...).
func (s *Sink) RecordsAtLevel(level string) ([]map[string]any, error) {
	all, err := s.Records()
	if err != nil {
		return nil, err
	}

	var matching []map[string]any
	for _, record := range all {
		if record["level"] == level {
			matching = append(matching, record)
		}
	}
	return matching, nil
}

// ErrorRecords returns the error-level records, which is what every
// recovered-panic assertion reads.
func (s *Sink) ErrorRecords() ([]map[string]any, error) {
	return s.RecordsAtLevel(slog.LevelError.String())
}
