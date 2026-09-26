package repository

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// CompletePeriodPageSize bounds one complete-period read to keep each future
// gRPC response and repository allocation below the transport budget.
const CompletePeriodPageSize int32 = 50

// ActivePeriodCursor identifies the last active row returned by a complete
// period read. The fields form a total order for ISO dates and timestamps.
type ActivePeriodCursor struct {
	ExpenseDate string
	CreatedAt   string
	ID          string
}

// GetActiveExpensesByPeriodAfter returns one bounded keyset page of active
// expenses for a user period. The page is ordered by expense_date, created_at,
// and id in ascending order. The extra row determines hasMore without COUNT.
func (r *ImmudbExpenseRepository) GetActiveExpensesByPeriodAfter(ctx context.Context, userID string, year, month int32, cursor ActivePeriodCursor, pageSize int32) ([]*model.Expense, ActivePeriodCursor, bool, error) {
	if pageSize < 1 || pageSize > CompletePeriodPageSize {
		pageSize = CompletePeriodPageSize
	}

	params := map[string]interface{}{
		"user_id": userID,
		"year":    year,
		"month":   month,
		"limit":   pageSize + 1,
	}

	cursorPredicate := ""
	if cursor.ExpenseDate != "" {
		cursorPredicate = ` AND (expense_date > @cursor_expense_date
		OR (expense_date = @cursor_expense_date AND created_at > @cursor_created_at)
		OR (expense_date = @cursor_expense_date AND created_at = @cursor_created_at AND id > @cursor_id))`
		params["cursor_expense_date"] = cursor.ExpenseDate
		params["cursor_created_at"] = cursor.CreatedAt
		params["cursor_id"] = cursor.ID
	}

	query := fmt.Sprintf(`SELECT %s FROM expenses
		WHERE user_id = @user_id
		AND period_year = @year
		AND period_month = @month
		AND status = 'active'%s
		ORDER BY expense_date ASC, created_at ASC, id ASC
		LIMIT @limit;`, expenseSelectColumns, cursorPredicate)

	result, err := r.client.SQLQuery(ctx, query, params)
	if err != nil {
		return nil, ActivePeriodCursor{}, false, fmt.Errorf("querying active expenses for period: %w", err)
	}

	rows := make([]*model.Expense, 0, len(result.Rows))
	for _, row := range result.Rows {
		expense, mapErr := r.mapRow(row)
		if mapErr != nil {
			return nil, ActivePeriodCursor{}, false, fmt.Errorf("mapping active period expense row: %w", mapErr)
		}
		rows = append(rows, expense)
	}

	hasMore := int32(len(rows)) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	next := cursor
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		next = ActivePeriodCursor{
			ExpenseDate: last.ExpenseDateIso,
			CreatedAt:   last.CreatedAt,
			ID:          last.ID,
		}
	}

	return rows, next, hasMore, nil
}
