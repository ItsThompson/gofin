# Backend Validation

## Overview

Backend request validation follows one shared, declarative pattern across every service: accumulate every field-level violation in a single pass and return them together, instead of short-circuiting on the first failure. The pattern lives in the shared [`services/shared/validator`](../services/shared/validator) package and is the only validation approach used in service-layer business-rule checks.

Validation has two distinct concerns, each with its own home:

| Concern | Where | Pattern |
|---------|-------|---------|
| **Business-rule validation** (field presence, ranges, enum membership, cross-field invariants like E/D/S summing to 100) | Service layer (`*/internal/service`) | Shared `validator` package - this doc |
| **Transport parsing** (query-param string→int coercion, JSON binding required-tags, body-field detection) | Handler layer (`*/internal/handler`) | Gin binding / inline parsing - different purpose, different response mechanism |

Business-rule validation returns an `*apierr.Error` up the call stack so the handler's `apierr.Respond` renders it. Transport parsing rejects malformed requests before the service is reached. Do not mix the two: a service method never parses query strings, and a handler never enforces business rules.

## The Shared Validator

`services/shared/validator/validator.go` is a tiny accumulator:

```go
v := validator.New()                          // empty validator
v.Check(cond, "fieldName", "field message")   // records msg for field when cond is false
v.Check(cond2, "fieldName2", "field message2")
if v.HasErrors() {
    return apierr.Validation("validation failed", v.Errors())
}
return nil
```

Two semantics make this work:

- **First error per field wins.** `Check` records a message for a field only the first time that field fails. Later checks for the same field are ignored. This lets you stack dependent checks (required, then format, then range) on one field without the later, more-specific check clobbering the earlier, more-relevant one - the most relevant failure surfaces.
- **All checks run in one pass.** Nothing short-circuits between checks, so every violated field appears in the result. A request with three bad fields gets all three back in one response, letting the caller fix them together instead of ping-ponging.

## The Canonical Pattern

Every service-layer validator follows this shape. The pro-rata validators in `services/expense/internal/service/prorata_request.go` are the reference implementation.

```go
func validateCreateExpenseRequest(req *model.CreateExpenseRequest) *apierr.Error {
    v := validator.New()
    v.Check(req.Name != "", "name", "name is required")
    v.Check(req.AmountInTransactionCurrencyMinorUnits > 0, "amountInTransactionCurrencyMinorUnits", "amount must be positive")
    v.Check(model.ValidExpenseTypes[req.ExpenseType], "expenseType", "expense_type must be one of: essentials, desires, savings")
    v.Check(req.TagID != "", "tagId", "tag_id is required")
    v.Check(req.ExpenseDateIso != "", "expenseDateIso", "expense_date is required")
    v.Check(isoDateRegex.MatchString(req.ExpenseDateIso), "expenseDateIso", "expense_date must be in ISO format (YYYY-MM-DD)")
    v.Check(req.PeriodYear >= 1, "periodYear", "period_year must be positive")
    v.Check(req.PeriodMonth >= 1 && req.PeriodMonth <= 12, "periodMonth", "period_month must be between 1 and 12")
    if v.HasErrors() {
        return apierr.Validation("validation failed", v.Errors())
    }
    return nil
}
```

### Single-field checks

A method that validates one input (e.g. an ID path parameter) inlines a one-check validator at the call site rather than hiding it in a helper:

```go
func (s *ExpenseService) GetExpense(ctx context.Context, userID string, id string) (*model.Expense, error) {
    v := validator.New()
    v.Check(id != "", "expenseId", "expense ID is required")
    if v.HasErrors() {
        return nil, apierr.Validation("validation failed", v.Errors())
    }
    // ...
}
```

### Dependent checks on one field

Stack checks in order of relevance; first-error-per-field makes the right message win:

```go
_, uuidErr := uuid.Parse(key)
v := validator.New()
v.Check(key != "", "clientGeneratedIdempotencyKey", "clientGeneratedIdempotencyKey is required")
v.Check(len(key) <= 36, "clientGeneratedIdempotencyKey", "clientGeneratedIdempotencyKey must be at most 36 characters")
v.Check(uuidErr == nil, "clientGeneratedIdempotencyKey", "clientGeneratedIdempotencyKey must be a valid UUID")
```

An empty key reports only "required" (the length check passes for `""` and the UUID check fails, but both are ignored once "required" is recorded). A too-long key reports only "at most 36 characters". A valid-length non-UUID reports only "must be a valid UUID". Same effective behavior as a short-circuit chain, but with field-level detail in the response.

## Preserving Custom Error Codes

Some validations carry a specific error code that callers branch on (e.g. `UNSUPPORTED_CURRENCY`). These do not use `apierr.Validation` (which always sets `Code: VALIDATION_ERROR`). Instead, run the check through the validator and build a custom `*apierr.Error` with `v.Errors()` as the `Fields` map - the wire contract is preserved (same code, same message format, same field detail):

```go
func validateSupportedCurrency(fieldName string, currencyCode string) *apierr.Error {
    v := validator.New()
    v.Check(currencycatalog.IsSupported(currencyCode), fieldName, "unsupported currency")
    if v.HasErrors() {
        return &apierr.Error{
            Code:    model.ErrUnsupportedCurrency,
            Message: fmt.Sprintf("Unsupported currency %q", currencyCode),
            Status:  http.StatusBadRequest,
            Fields:  v.Errors(),
        }
    }
    return nil
}
```

Rule: use `apierr.Validation("validation failed", v.Errors())` for plain validation errors. Build a custom `*apierr.Error` only when a typed code other than `VALIDATION_ERROR` is part of the contract.

## Wire Contract

Validation errors render through `apierr.Respond` as the standard error shape (see [Response Contracts in api.md](api.md#response-contracts)):

```json
{
  "code": "VALIDATION_ERROR",
  "message": "validation failed",
  "fields": {
    "name": "name is required",
    "amountInTransactionCurrencyMinorUnits": "amount must be positive",
    "expenseType": "expense_type must be one of: essentials, desires, savings"
  }
}
```

Conventions:

- **Top-level `message`** is the standardized `"validation failed"` for every validation error. It is intentionally generic - the specific, displayable detail lives in the `fields` map. This keeps the wire contract uniform across services and matches the pro-rata reference pattern.
- **`fields`** maps the request field name (matching the JSON tag / wire field the client sent) to a human-readable message. Every field that failed a check appears; first-error-per-field means each field has exactly one message.
- **Field names** match the wire field names (`name`, `expenseType`, `periodMonth`), not Go struct field names or snake_case domain terms - except where the established contract already uses a specific form (e.g. `tag_id is required` on the `tagId` field, preserved for backward compatibility).

## What Not to Do

- **No hand-rolled `fields := make(map[string]string)` accumulation.** The shared validator is the only way to build the `fields` map in service-layer *validation*. Two non-validation sites build field maps directly and are expected: the handler's `immutableReportingCurrencyFields` (transport-layer body-field detection) and `fx_client.go`'s `mapFxError` INVALID_AMOUNT branch (an error *mapper* translating an upstream gRPC rejection - there is no condition to evaluate, so forcing it through `validator.New()` would reintroduce the `v.Check(false, ...)` anti-pattern). A `grep -rn "fields := make(map\[string\]string)" services/ --include="*.go" | grep -v shared/validator | grep -v _test.go` should return nothing.
- **No `apierr.Validation("msg", nil)` with a nil fields map.** Every validation call carries field-level detail via `v.Errors()`. A nil fields map is a missing-detail bug.
- **No `v.Check(false, ...)`.** The condition belongs at the check site. A constant `false` makes the validator a verbose map builder and leaves dead code behind (`return nil` is unreachable). If the condition is already checked at the call site, inline the validator there instead of hiding it in a helper.
- **No short-circuiting between validation tiers.** If a request has multiple kinds of violations (negative values *and* a sum mismatch), surface all of them. The caller should fix everything in one round-trip. `ValidateEDSSplit` is the reference: non-negative, not-over-100, and sum-to-100 checks all run in one validator.
- **No service-layer query-param parsing.** Parsing `year`/`month`/`page` strings is the handler's job. Service methods receive typed values and validate the business rules (range, presence), not the transport.
- **No top-level messages more specific than "validation failed".** Specific messages go in the `fields` entry for the offending field. A reviewer should not see `apierr.Validation("E/D/S percentages must be non-negative", ...)` - that detail belongs in the `essentialsPercent` field message.

## Where Validation Lives

| Service | File | Validators |
|---------|------|------------|
| Expense | `services/expense/internal/service/expense.go` | `validateCreateExpenseRequest`, `validateCorrectExpenseRequest`, `validateIdempotencyKey`, `validateTransactionCurrency` (custom `ErrUnsupportedCurrency` code), and inline checks on `GetExpense`, `CorrectExpense`, `DeleteExpense`, `GetCorrectionHistory`, `GetExpensesInProRataGroup`, `CountExpensesByTag`, `GetActiveExpensesForPeriod`, `StreamAllUserExpenses`, `AnonymizeAllUserExpenses` |
| Expense | `services/expense/internal/service/suggestions.go` | `GetExpenseSuggestions` (user/page/pageSize) |
| Expense | `services/expense/internal/service/prorata_request.go` | `validateProRataInstallmentRequest`, `validateTrustedPeriodContext`, `validateSnapshotCoverage` (reference implementation) |
| Finance | `services/finance/internal/service/finance.go` | `ValidateEDSSplit` (aggregated), `validateSupportedCurrency` (custom `ErrUnsupportedCurrency` code), `validateTagName`, and the inline `budgetAmount` checks in `UpdateDefaults`/`UpdatePeriod` and the `GetCurrentPeriod` month check |
| Finance | `services/finance/internal/service/prorata.go` | `CreateProRataExpense` request checks, `CreatePeriodWithProRata` month/budget checks |

## Testing Validators

Validator tests assert on `verr.Fields`, not `verr.Message`. The top-level message is always `"validation failed"` and carries no specific information; the field-level messages are the contract worth locking in. See `TestValidateEDSSplit_Invalid` / `TestValidateEDSSplit_FieldsPopulated` in `services/finance/internal/service/finance_test.go` and `expense_validation_test.go` for the pattern:

- **Valid case** → returns `nil`.
- **Each field invalid in isolation** → the right field appears with the right message.
- **Multiple fields invalid** → all appear (aggregation); first-error-per-field means each has exactly one message.

Run the relevant suite with:

```bash
cd services
go test ./expense/internal/service/... -run TestValidate
go test ./finance/internal/service/... -run TestValidateEDSSplit
```
