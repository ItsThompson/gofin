# Testing

## Philosophy

Each service is independently testable with clear boundaries. The strategy prioritizes:

1. **Deep modules get exhaustive tests**: correction logic, materialization, budget lifecycle, token lifecycle, and RBAC
2. **Test behavior, not implementation**: verify what a function returns or what side effect it produces
3. **Mock at service boundaries**: each service mocks external dependencies at the interface level
4. **Frontend tests focus on user interactions**: what the user sees and does, not internal component state

## Test Layers

| Layer | Scope | Tooling | Mocks | Command |
|-------|-------|---------|-------|---------|
| Go unit tests | Single function/method | `testing` + `testify` | DB repos, gRPC clients | `just test-backend` |
| Go integration tests | Service + real database | `testing` + `testcontainers-go` | Other services (gRPC) | `go test -run Integration ./...` |
| Frontend unit tests | Component or hook | Vitest + React Testing Library | API calls (MSW) | `just test-frontend` |
| Frontend integration tests | Page-level rendering | Vitest + RTL | API calls (MSW) | `just test-frontend` |
| E2E tests | Full user journey | Playwright | Nothing (full stack) | `just test-e2e` |

## Running Tests

```bash
# All tests (backend + frontend)
just test

# Backend only (Go tests across all services)
just test-backend

# Frontend only (via Turborepo)
just test-frontend

# E2E (requires running stack: `just up` + `just seed-admin` first)
just test-e2e

# Skip integration tests (Go)
cd services/auth && go test -short ./...
```

## Go Testing Patterns

### Table-Driven Tests

```go
func TestCalculateInstallmentAmounts(t *testing.T) {
    tests := []struct {
        name          string
        totalCents    int64
        months        int
        wantFirst     int64
        wantRemaining int64
    }{
        {"evenly divisible", 9000, 3, 3000, 3000},
        {"remainder absorbed by first", 10000, 3, 3400, 3300},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            first, remaining := CalculateInstallmentAmounts(tt.totalCents, tt.months)
            assert.Equal(t, tt.wantFirst, first)
            assert.Equal(t, tt.wantRemaining, remaining)
        })
    }
}
```

### Interface-Based Mocking

Each service defines interfaces for its dependencies. Tests use `testify/mock`:

```go
type ExpenseRepository interface {
    Create(ctx context.Context, entry model.Expense) (model.Expense, error)
    GetByID(ctx context.Context, id string) (model.Expense, error)
    GetActiveForPeriod(ctx context.Context, userID string, year, month int) ([]model.Expense, error)
}

type MockExpenseRepository struct { mock.Mock }
// ... mock method implementations
```

### Integration Tests with Testcontainers

Integration tests spin up real PostgreSQL/immudb instances via `testcontainers-go`. They are skipped with `-short`:

```go
func TestExpenseRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }
    // ... spin up container, run real queries
}
```

## High-Value Test Targets

### Backend

| Component | Priority | Focus |
|-----------|----------|-------|
| Expense: correction logic | High | Chain creation, atomic status updates, rejection of already-corrected entries |
| Expense: materialization | High | Active-only filtering, correction chain resolution |
| Finance: budget lifecycle | High | Period creation, missed months, E/D/S calculation, rounding |
| Finance: pro-rata scheduling | High | Installment math, remainder handling, year rollover, application at period creation |
| Auth: token lifecycle | High | Generation, validation, refresh rotation, blacklisting, `tokens_revoked_at` |
| Auth: RBAC | High | Role checking, identity assumption, audit claims |
| Gateway: auth middleware | High | Valid token pass-through, expired token rejection |
| Finance: aggregations | Medium | Category sums, tag spending, cumulative spend |
| Auth: password handling | Medium | Hashing, verification, strength validation |
| Gateway: routing | Medium | Route matching, header forwarding |

### Frontend

| Component | Priority | Focus |
|-----------|----------|-------|
| Auth store | High | Login, logout, refresh, assumption state transitions |
| Auth flow (login/register) | High | Form validation, error messages, redirect |
| ExpenseForm | High | Validation, pro-rata toggle, submission |
| ExpenseLog (TanStack Table) | Medium | Sorting, filtering, pagination |
| Dashboard gauges | Medium | Percentage display, color coding |
| Onboarding flow | Medium | Step progression, skip behavior |
| Settings page | Medium | E/D/S sum validation, tag CRUD |
| Admin panel | Medium | User list, assume action, role gating |

## Frontend Testing Patterns

### Component Tests with Vitest + RTL

Tests use React Testing Library with `userEvent` for realistic interactions:

```typescript
describe('ExpenseForm', () => {
    it('submits a standard expense', async () => {
        const onSubmit = vi.fn();
        render(<ExpenseForm onSubmit={onSubmit} tags={mockTags} />);
        await userEvent.type(screen.getByLabelText('Name'), 'Groceries');
        await userEvent.click(screen.getByRole('button', { name: 'Save' }));
        expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ name: 'Groceries' }));
    });
});
```

### API Mocking with MSW

```typescript
export const handlers = [
    http.get('/api/budget/current', () => HttpResponse.json(mockPeriodSummary)),
    http.get('/api/expenses', ({ request }) => {
        const url = new URL(request.url);
        const page = url.searchParams.get('page') ?? '1';
        return HttpResponse.json(mockExpenseList(Number(page)));
    }),
];
```

## E2E Test Scenarios (Playwright)

| Scenario | Steps |
|----------|-------|
| Registration → Onboarding → First Expense | Register, complete onboarding, confirm new month, log expense, verify dashboard |
| Expense correction | Log expense, open detail, correct it, verify correction in modal |
| Pro-rata creation | Log 3-month pro-rata, verify current month entry, advance month, verify auto-applied installment |
| Budget period transition | Complete a month, advance, verify prompt, confirm defaults |
| Admin identity assumption | Login as admin, assume user, verify data switch, return to admin |
| Mobile expense logging | Mobile viewport, login, log expense, verify in log |
