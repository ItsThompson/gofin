package clients

import (
	"context"
	"errors"
	"testing"

	"github.com/ItsThompson/gofin/services/reporting/internal/config"
)

func TestDialRejectsMalformedServiceAddressWithoutExposingIt(t *testing.T) {
	_, err := Dial(context.Background(), config.Config{
		AuthAddr:       "malformed",
		ExpenseAddr:    "expense-service:9082",
		FinanceAddr:    "finance-service:9083",
		DatarightsAddr: "datarights-service:9084",
	})
	if err == nil {
		t.Fatal("expected malformed address to fail")
	}
	var dialErr *DialError
	if !errors.As(err, &dialErr) || dialErr.Service != "auth" {
		t.Fatalf("error = %T %+v", err, err)
	}
	if got := err.Error(); got != "create auth client failed" {
		t.Fatalf("error text = %q", got)
	}
}

func TestDialHonorsCanceledContextWithoutCreatingClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Dial(ctx, config.Config{
		AuthAddr:       "bad-auth",
		ExpenseAddr:    "bad-expense",
		FinanceAddr:    "bad-finance",
		DatarightsAddr: "bad-datarights",
	})
	if err == nil {
		t.Fatal("expected canceled dial to fail")
	}
	var dialErr *DialError
	if !errors.As(err, &dialErr) || dialErr.Service != "auth" {
		t.Fatalf("error = %T %+v", err, err)
	}
	if got := err.Error(); got != "create auth client failed" {
		t.Fatalf("error text = %q", got)
	}
}
