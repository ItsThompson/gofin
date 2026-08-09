// Package errkittest provides the recording sentry.Transport the errkit tests
// use, exported so serverkit and the migrated handler tests assert against the
// same seam.
//
// A transport stub is the right seam because it exercises the real SDK path,
// including event normalization, exception-chain walking, and stack extraction,
// which are exactly the behaviors errkit's design depends on. It needs no DSN and
// no network, and it makes the assertions about the event, which is the contract,
// rather than about a call to our own function.
package errkittest

import (
	"context"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

// hermeticDSN is syntactically valid so the client initializes as enabled, and
// unroutable so a stub that is accidentally replaced by the real HTTP transport
// fails loudly instead of reaching Sentry. Nothing is sent while Transport is
// installed: the SDK hands every event straight to it.
const hermeticDSN = "https://public@example.invalid/1"

// Transport records every event the SDK would have sent. It implements all five
// sentry.Transport methods, which ClientOptions.Transport requires.
type Transport struct {
	mu     sync.Mutex
	events []*sentry.Event
}

// SendEvent records event instead of sending it. It is safe to call from many
// goroutines, which the errkit concurrency test relies on.
func (t *Transport) SendEvent(event *sentry.Event) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, event)
}

func (t *Transport) Flush(time.Duration) bool              { return true }
func (t *Transport) FlushWithContext(context.Context) bool { return true }
func (t *Transport) Configure(sentry.ClientOptions)        {}
func (t *Transport) Close()                                {}

// Events returns a snapshot of the recorded events, oldest first.
func (t *Transport) Events() []*sentry.Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]*sentry.Event(nil), t.events...)
}

// NewClient returns a client that records into tr. Logs and metrics are disabled
// because their batch processors start background goroutines a test has no use
// for.
//
// Each configure function runs against the options before the client is built, so
// a test needing an init-level setting such as ClientOptions.Tags can add it
// without restating the defaults. The three settings that make the client
// hermetic are re-asserted afterwards, so no configure function can undo them.
//
// It panics if the client cannot be built. With no configure function the only
// failure mode is an unparseable DSN and the DSN is a constant, so a panic means a
// configure function supplied a bad option.
func NewClient(tr *Transport, configure ...func(*sentry.ClientOptions)) *sentry.Client {
	options := sentry.ClientOptions{
		Dsn:            hermeticDSN,
		DisableLogs:    true,
		DisableMetrics: true,
	}
	for _, apply := range configure {
		apply(&options)
	}

	// Transport keeps every event inside the process and inside tr. DataCollection
	// stays nil because a non-nil value, even an empty one, turns user info on and
	// collects every HTTP body type, cookie, and header. SendDefaultPII is deprecated
	// but not inert here: it is the input to the legacy resolution that a nil
	// DataCollection selects, so a hook setting it would reach the same permissive
	// posture by the other door.
	options.Transport = tr
	options.DataCollection = nil
	options.SendDefaultPII = false //nolint:staticcheck // pinned precisely because a configure hook can still set it

	client, err := sentry.NewClient(options)
	if err != nil {
		panic("errkittest: building the recording client: " + err.Error())
	}
	return client
}

// ContextWithHub returns ctx carrying its own hub bound to a client that records
// into tr. This is the shape production has, because sentrygin and sentrygrpc
// install a per-request hub on the context, so it exercises the
// GetHubFromContext path rather than the clone fallback.
func ContextWithHub(ctx context.Context, tr *Transport, configure ...func(*sentry.ClientOptions)) context.Context {
	return sentry.SetHubOnContext(ctx, sentry.NewHub(NewClient(tr, configure...), sentry.NewScope()))
}
