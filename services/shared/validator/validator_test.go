package validator

import "testing"

func TestNew_HasNoErrors(t *testing.T) {
	v := New()

	if v.HasErrors() {
		t.Fatal("New validator reports errors before any check")
	}
	if len(v.Errors()) != 0 {
		t.Fatalf("New validator errors = %v, want empty", v.Errors())
	}
}

func TestCheck_RecordsErrorForFailedCondition(t *testing.T) {
	v := New()

	v.Check(false, "amount", "amount is required")

	if !v.HasErrors() {
		t.Fatal("HasErrors = false after a failed check, want true")
	}
	want := map[string]string{"amount": "amount is required"}
	if got := v.Errors(); got["amount"] != want["amount"] || len(got) != 1 {
		t.Fatalf("Errors = %v, want %v", got, want)
	}
}

func TestCheck_IgnoresPassingCondition(t *testing.T) {
	v := New()

	v.Check(true, "amount", "amount is required")

	if v.HasErrors() {
		t.Fatal("HasErrors = true after a passing check, want false")
	}
	if len(v.Errors()) != 0 {
		t.Fatalf("Errors = %v, want empty", v.Errors())
	}
}

func TestCheck_FirstErrorPerFieldWins(t *testing.T) {
	v := New()

	v.Check(false, "expenseDate", "expense_date is required")
	v.Check(false, "expenseDate", "expense_date must be in ISO format (YYYY-MM-DD)")

	errors := v.Errors()
	if len(errors) != 1 {
		t.Fatalf("Errors = %v, want exactly one entry", errors)
	}
	if got := errors["expenseDate"]; got != "expense_date is required" {
		t.Fatalf("expenseDate error = %q, want the first recorded message", got)
	}
}

func TestCheck_AccumulatesDistinctFields(t *testing.T) {
	v := New()

	v.Check(false, "name", "name is required")
	v.Check(false, "amount", "amount must be positive")

	errors := v.Errors()
	if len(errors) != 2 {
		t.Fatalf("Errors = %v, want two entries", errors)
	}
	if errors["name"] != "name is required" {
		t.Fatalf("name error = %q, want %q", errors["name"], "name is required")
	}
	if errors["amount"] != "amount must be positive" {
		t.Fatalf("amount error = %q, want %q", errors["amount"], "amount must be positive")
	}
}
