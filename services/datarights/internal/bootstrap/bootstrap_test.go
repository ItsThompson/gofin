package bootstrap

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/datarights/internal/config"
)

func TestRunWithDependencies_BindFailureSuppressesEveryStartupSideEffect(t *testing.T) {
	bindErr := errors.New("address already in use")
	cfg := &config.Config{GRPCPort: config.DefaultGRPCPort}
	var migrations, recovery, email, serving int

	// The starter is the injected boundary for migrations, recovery, email,
	// and serving. None may run when binding fails.
	err := runWithDependencies(
		context.Background(), cfg,
		func(string, string) (net.Listener, error) { return nil, bindErr },
		func(context.Context, *config.Config, net.Listener) error {
			migrations++
			recovery++
			email++
			serving++
			return nil
		},
	)

	require.ErrorIs(t, err, bindErr)
	assert.Zero(t, migrations)
	assert.Zero(t, recovery)
	assert.Zero(t, email)
	assert.Zero(t, serving)
}

func TestRunWithDependencies_ClosesListenerBeforeStartErrorReturns(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()

	startErr := errors.New("database unavailable")
	err = runWithDependencies(
		context.Background(), &config.Config{GRPCPort: config.DefaultGRPCPort},
		func(string, string) (net.Listener, error) { return listener, nil },
		func(context.Context, *config.Config, net.Listener) error { return startErr },
	)

	require.ErrorIs(t, err, startErr)
	rebound, reboundErr := net.Listen("tcp", address)
	require.NoError(t, reboundErr)
	require.NoError(t, rebound.Close())
}

func TestRunWithDependencies_BindsInternalGRPCPort(t *testing.T) {
	cfg := &config.Config{GRPCPort: "19084"}
	var network, address string
	bindErr := errors.New("stop after bind")

	err := runWithDependencies(
		context.Background(), cfg,
		func(gotNetwork, gotAddress string) (net.Listener, error) {
			network = gotNetwork
			address = gotAddress
			return nil, bindErr
		},
		func(context.Context, *config.Config, net.Listener) error { return nil },
	)

	require.ErrorIs(t, err, bindErr)
	assert.Equal(t, "tcp", network)
	assert.Equal(t, ":19084", address)
}
