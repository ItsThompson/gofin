package clients

import (
	"context"
	"net"
	"strconv"
	"strings"

	authpb "github.com/ItsThompson/gofin/services/auth/proto/authpb"
	datarightspb "github.com/ItsThompson/gofin/services/datarights/proto/datarightspb"
	expensepb "github.com/ItsThompson/gofin/services/expense/proto/expensepb"
	financepb "github.com/ItsThompson/gofin/services/finance/proto/financepb"
	"github.com/ItsThompson/gofin/services/reporting/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Set struct {
	Auth        authpb.AuthServiceClient
	Expense     expensepb.ExpenseServiceClient
	Finance     financepb.FinanceServiceClient
	Datarights  datarightspb.DatarightsServiceClient
	connections []*grpc.ClientConn
}

func Dial(ctx context.Context, cfg config.Config) (Set, error) {
	addresses := []struct {
		name string
		addr string
	}{
		{name: "auth", addr: cfg.AuthAddr},
		{name: "expense", addr: cfg.ExpenseAddr},
		{name: "finance", addr: cfg.FinanceAddr},
		{name: "datarights", addr: cfg.DatarightsAddr},
	}
	connections := make([]*grpc.ClientConn, 0, len(addresses))
	closeConnections := func() {
		for _, connection := range connections {
			_ = connection.Close()
		}
	}
	for _, service := range addresses {
		if err := ctx.Err(); err != nil {
			closeConnections()
			return Set{}, &DialError{Service: service.name}
		}
		if !validAddress(service.addr) {
			closeConnections()
			return Set{}, &DialError{Service: service.name}
		}
		connection, err := grpc.NewClient(service.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			closeConnections()
			return Set{}, &DialError{Service: service.name}
		}
		connections = append(connections, connection)
	}
	if err := ctx.Err(); err != nil {
		closeConnections()
		return Set{}, &DialError{Service: "services"}
	}
	return Set{
		Auth:        authpb.NewAuthServiceClient(connections[0]),
		Expense:     expensepb.NewExpenseServiceClient(connections[1]),
		Finance:     financepb.NewFinanceServiceClient(connections[2]),
		Datarights:  datarightspb.NewDatarightsServiceClient(connections[3]),
		connections: connections,
	}, nil
}

func validAddress(address string) bool {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || host == "" {
		return false
	}
	portNumber, err := strconv.Atoi(port)
	return err == nil && portNumber > 0 && portNumber <= 65535
}

type DialError struct {
	Service string
}

func (e *DialError) Error() string {
	if e == nil {
		return ""
	}
	return "create " + e.Service + " client failed"
}

func (s Set) Close() error {
	var first error
	for _, connection := range s.connections {
		if err := connection.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
