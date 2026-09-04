package repository

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// GetExpensesByUserAfter returns one keyset page of expenses (active +
// corrected) for a user past the given cursor, ordered by
// (created_at ASC, id ASC). It seeks with the expanded-OR (created_at, id)
// predicate instead of LIMIT/OFFSET and derives hasMore by fetching pageSize+1
// rows and inspecting the overflow row, so it issues no OFFSET and no per-page
// COUNT(*). An empty cursor starts from the beginning.
//
// immudb 1.11.0 does not support SQL row-value tuple syntax
// ((created_at, id) > (@c, @cid) raises a syntax error), so the comparison is
// written in expanded-OR form.
func (r *ImmudbExpenseRepository) GetExpensesByUserAfter(ctx context.Context, userID string, cursor ExpenseCursor, pageSize int32) ([]*model.Expense, ExpenseCursor, bool, error) {
	if pageSize < 1 {
		pageSize = DefaultStreamPageSize
	}

	// Fetch one extra row (pageSize+1) so the overflow row reveals whether more
	// rows remain, avoiding a per-page COUNT(*).
	params := map[string]interface{}{
		"user_id": userID,
		"limit":   pageSize + 1,
	}

	cursorPredicate := ""
	if cursor.CreatedAt != "" {
		cursorPredicate = ` AND (created_at > @cursor_created_at
		OR (created_at = @cursor_created_at AND id > @cursor_id))`
		params["cursor_created_at"] = cursor.CreatedAt
		params["cursor_id"] = cursor.ID
	}

	dataQuery := fmt.Sprintf(`SELECT %s FROM expenses
		WHERE user_id = @user_id%s
		ORDER BY created_at ASC, id ASC
		LIMIT @limit;`, expenseSelectColumns, cursorPredicate)

	result, err := r.client.SQLQuery(ctx, dataQuery, params)
	if err != nil {
		return nil, ExpenseCursor{}, false, fmt.Errorf("querying user expenses after cursor: %w", err)
	}

	rows := make([]*model.Expense, 0, len(result.Rows))
	for _, row := range result.Rows {
		expense, convErr := r.mapRow(row)
		if convErr != nil {
			return nil, ExpenseCursor{}, false, fmt.Errorf("mapping expense row: %w", convErr)
		}
		rows = append(rows, expense)
	}

	// The overflow row (pageSize+1th) means more rows remain. Drop it from the
	// page and report hasMore.
	hasMore := int32(len(rows)) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	next := cursor
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		next = ExpenseCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}

	return rows, next, hasMore, nil
}
