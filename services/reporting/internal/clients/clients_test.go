package clients

import (
	"context"
	"errors"
	"testing"

	"github.com/ItsThompson/gofin/services/reporting/internal/config"
)

func TestDialRejectsMalformedServiceAddressWithoutExposingIt(t *testing.T) {
	_, err := Dial(context.Background(), config.Config{
		AuthAddr:       "foo/bar:9081",
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

func TestValidAddressRejectsTargetSyntax(t *testing.T) {
	for _, address := range []string{
		"foo/bar:9081",
		"https://foo:9081",
		"foo bar:9081",
		"foo:9081/path",
		"foo: +1",
		"foo:9081 ",
		"foo:0",
		"foo:65536",
		"-foo:9081",
		"foo..bar:9081",
	} {
		if validAddress(address) {
			t.Errorf("validAddress(%q) = true", address)
		}
	}
	for _, address := range []string{"auth-service:9081", "127.0.0.1:9081", "[::1]:9081"} {
		if !validAddress(address) {
			t.Errorf("validAddress(%q) = false", address)
		}
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
