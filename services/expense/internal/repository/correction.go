package repository

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ItsThompson/gofin/services/expense/internal/model"
)

// CorrectExpense atomically marks the original expense as "corrected" and
// inserts a new correction entry. immudb's SQL interface does not support
// multi-statement ExecAll; each statement is individually atomic in its MVCC
// model. For a single-user personal finance app, sequential execution is safe.
func (r *ImmudbExpenseRepository) CorrectExpense(ctx context.Context, original *model.Expense, correction *model.Expense) (*model.Expense, error) {
	updateQuery := `UPDATE expenses SET status = 'corrected' WHERE id = @id;`
	_, err := r.client.SQLExec(ctx, updateQuery, map[string]interface{}{
		"id": original.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("marking original expense as corrected: %w", err)
	}

	created, err := r.CreateExpense(ctx, correction)
	if err != nil {
		return nil, fmt.Errorf("inserting correction entry: %w", err)
	}

	r.logger.Info("expense corrected",
		slog.String("original_id", original.ID),
		slog.String("correction_id", created.ID),
	)

	return created, nil
}

// GetCorrectionHistory returns the full correction chain for an expense.
// It finds the root of the chain (the original entry with no corrects_id),
// then collects all entries that form the chain in chronological order.
func (r *ImmudbExpenseRepository) GetCorrectionHistory(ctx context.Context, expenseID string, userID string) ([]*model.Expense, error) {
	// First, fetch the requested expense
	starting, err := r.GetExpenseByID(ctx, expenseID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetching starting expense: %w", err)
	}
	if starting == nil {
		return nil, nil
	}

	// Walk backwards to the root (original) via corrects_id
	root := starting
	visited := map[string]bool{root.ID: true}
	for root.CorrectsID != "" {
		parent, parentErr := r.GetExpenseByID(ctx, root.CorrectsID, userID)
		if parentErr != nil {
			return nil, fmt.Errorf("walking correction chain: %w", parentErr)
		}
		if parent == nil {
			break
		}
		if visited[parent.ID] {
			break // Safety: prevent infinite loops
		}
		visited[parent.ID] = true
		root = parent
	}

	// Now collect the chain from root forward:
	// root -> correction1 -> correction2 -> ...
	chain := []*model.Expense{root}
	currentID := root.ID

	for {
		// Find the entry that corrects the current one
		nextQuery := fmt.Sprintf(`SELECT %s FROM expenses
			WHERE corrects_id = @corrects_id AND user_id = @user_id;`, expenseSelectColumns)

		result, queryErr := r.client.SQLQuery(ctx, nextQuery, map[string]interface{}{
			"corrects_id": currentID,
			"user_id":     userID,
		})
		if queryErr != nil {
			return nil, fmt.Errorf("following correction chain forward: %w", queryErr)
		}

		if len(result.Rows) == 0 {
			break
		}

		next, convErr := r.mapRow(result.Rows[0])
		if convErr != nil {
			return nil, fmt.Errorf("mapping expense row: %w", convErr)
		}
		if visited[next.ID] {
			break // Safety
		}
		visited[next.ID] = true
		chain = append(chain, next)
		currentID = next.ID
	}

	return chain, nil
}
