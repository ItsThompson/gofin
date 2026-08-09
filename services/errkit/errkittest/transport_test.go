package errkittest_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/errkit/errkittest"
)

// A three-method stub would not compile against ClientOptions.Transport, so this
// pins all five methods of the interface rather than trusting the assignment in
// NewClient to keep covering them.
var _ sentry.Transport = (*errkittest.Transport)(nil)

func TestTransport_RecordsCapturedEvents(t *testing.T) {
	transport := &errkittest.Transport{}
	ctx := errkittest.ContextWithHub(context.Background(), transport)
	hub := sentry.GetHubFromContext(ctx)
	require.NotNil(t, hub)

	hub.CaptureException(errors.New("boom"))

	events := transport.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "boom", events[0].Exception[0].Value)
}

func TestTransport_EventsReturnsASnapshot(t *testing.T) {
	transport := &errkittest.Transport{}

	transport.SendEvent(sentry.NewEvent())
	snapshot := transport.Events()
	transport.SendEvent(sentry.NewEvent())

	assert.Len(t, snapshot, 1)
	assert.Len(t, transport.Events(), 2)
}

func TestTransport_LifecycleMethodsAreInert(t *testing.T) {
	transport := &errkittest.Transport{}

	transport.Configure(sentry.ClientOptions{})
	transport.Close()

	assert.True(t, transport.Flush(time.Second))
	assert.True(t, transport.FlushWithContext(context.Background()))
	assert.Empty(t, transport.Events())
}

// The errkit concurrency test reports from many goroutines through one client, so
// SendEvent must be safe to call concurrently.
func TestTransport_SendEventIsConcurrencySafe(t *testing.T) {
	const senders = 64

	transport := &errkittest.Transport{}

	var wg sync.WaitGroup
	wg.Add(senders)
	for range senders {
		go func() {
			defer wg.Done()
			transport.SendEvent(sentry.NewEvent())
		}()
	}
	wg.Wait()

	assert.Len(t, transport.Events(), senders)
}
