package clients

import (
	"context"
	"net"
	"strconv"
	"strings"
	"unicode"

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
			return Set{}, &DialError{Service: service.name}
		}
		if !validAddress(service.addr) {
			return Set{}, &DialError{Service: service.name}
		}
	}
	for _, service := range addresses {
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
	if address == "" || address != strings.TrimSpace(address) || strings.IndexFunc(address, unicode.IsSpace) >= 0 {
		return false
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || !validHost(host) || !validPort(port) {
		return false
	}
	portNumber, err := strconv.Atoi(port)
	return err == nil && portNumber > 0 && portNumber <= 65535
}

func validHost(host string) bool {
	if host == "" || strings.ContainsAny(host, "/\\?#%@") {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' {
				return false
			}
		}
	}
	return true
}

func validPort(port string) bool {
	if port == "" {
		return false
	}
	for _, char := range port {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
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
