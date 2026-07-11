// This file provides a local development stub for the immudb client.
// In the Docker build, the real immudb client SDK is available and this
// file is replaced by immudb_prod.go via build tags.
//
// For local development and testing, the service starts but cannot
// connect to immudb. All business logic is tested via the mock
// repository in unit tests.

//go:build !docker

package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
)

// inMemoryImmudbClient is a simple in-memory implementation of ImmudbClient
// for local development. It stores expenses in memory and supports basic
// SQL-like operations for the queries used by the expense service.
type inMemoryImmudbClient struct {
	mu   sync.RWMutex
	rows []map[string]interface{}
}

func newImmudbClientImpl(_ context.Context, _ *config.Config) (repository.ImmudbClient, error) {
	return &inMemoryImmudbClient{
		rows: make([]map[string]interface{}, 0),
	}, nil
}

func (c *inMemoryImmudbClient) SQLExec(_ context.Context, sql string, params map[string]interface{}) (*repository.SQLResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sqlLower := strings.ToLower(strings.TrimSpace(sql))

	if strings.HasPrefix(sqlLower, "create table") || strings.HasPrefix(sqlLower, "create index") {
		// Schema operations: no-op in memory
		return &repository.SQLResult{}, nil
	}

	if strings.HasPrefix(sqlLower, "insert into") {
		if params == nil {
			return nil, fmt.Errorf("INSERT requires parameters")
		}
		row := make(map[string]interface{}, len(params))
		for k, v := range params {
			row[k] = v
		}
		c.rows = append(c.rows, row)
		return &repository.SQLResult{}, nil
	}

	// UPDATE is not implemented by this in-memory stub. Falling through to a
	// silent empty result would let a local CorrectExpense leave the original
	// row active (double-counting) and make anonymize a silent no-op, so fail
	// loudly instead. The real immudb client (immudb_prod.go) handles UPDATE.
	if strings.HasPrefix(sqlLower, "update") {
		return nil, fmt.Errorf("UPDATE is unsupported in the local in-memory immudb stub")
	}

	return &repository.SQLResult{}, nil
}

func (c *inMemoryImmudbClient) SQLQuery(_ context.Context, sql string, params map[string]interface{}) (*repository.SQLResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sqlLower := strings.ToLower(strings.TrimSpace(sql))

	// COUNT query
	if strings.Contains(sqlLower, "count(*)") {
		count := int64(0)
		for _, row := range c.rows {
			if matchesWhere(row, params) {
				count++
			}
		}
		return &repository.SQLResult{
			Rows: []repository.SQLRow{
				{Values: []repository.SQLValue{&simpleValue{intVal: count}}},
			},
		}, nil
	}

	// SELECT by ID (with user scoping)
	if strings.Contains(sqlLower, "where id = @id") {
		id, _ := params["id"].(string)
		userID, _ := params["user_id"].(string)
		for _, row := range c.rows {
			rowID := fmt.Sprintf("%v", row["id"])
			rowUserID := fmt.Sprintf("%v", row["user_id"])
			if rowID == id && (userID == "" || rowUserID == userID) {
				return &repository.SQLResult{
					Rows: []repository.SQLRow{rowToSQLRow(row)},
				}, nil
			}
		}
		return &repository.SQLResult{}, nil
	}

	// SELECT with user_id/period filters
	var matched []map[string]interface{}
	for _, row := range c.rows {
		if matchesWhere(row, params) {
			matched = append(matched, row)
		}
	}

	// Apply LIMIT and OFFSET
	offset := int32(0)
	limit := int32(len(matched))
	if v, ok := params["offset"]; ok {
		offset = toInt32(v)
	}
	if v, ok := params["limit"]; ok {
		limit = toInt32(v)
	}

	start := int(offset)
	if start > len(matched) {
		start = len(matched)
	}
	end := start + int(limit)
	if end > len(matched) {
		end = len(matched)
	}
	paged := matched[start:end]

	result := &repository.SQLResult{
		Rows: make([]repository.SQLRow, 0, len(paged)),
	}
	for _, row := range paged {
		result.Rows = append(result.Rows, rowToSQLRow(row))
	}

	return result, nil
}

func matchesWhere(row, params map[string]interface{}) bool {
	if userID, ok := params["user_id"]; ok {
		if fmt.Sprintf("%v", row["user_id"]) != fmt.Sprintf("%v", userID) {
			return false
		}
	}
	if year, ok := params["year"]; ok {
		if toInt32(row["period_year"]) != toInt32(year) {
			return false
		}
	}
	if month, ok := params["month"]; ok {
		if toInt32(row["period_month"]) != toInt32(month) {
			return false
		}
	}
	// Filter active only (the queries in the repo always filter status='active')
	if status, ok := row["status"]; ok {
		if fmt.Sprintf("%v", status) != "active" {
			return false
		}
	}
	return true
}

func toInt32(v interface{}) int32 {
	switch val := v.(type) {
	case int32:
		return val
	case int64:
		return int32(val)
	case int:
		return int32(val)
	default:
		return 0
	}
}

// rowToSQLRow converts a map row to a SQLRow with values in the standard
// column order used by the SELECT queries in the repository.
func rowToSQLRow(row map[string]interface{}) repository.SQLRow {
	columns := []string{
		"id", "user_id", "name", "amount", "currency", "expense_type", "tag_id",
		"expense_date", "period_year", "period_month", "status", "corrects_id",
		"is_pro_rata", "pro_rata_group", "pro_rata_index", "pro_rata_total", "created_at",
	}

	values := make([]repository.SQLValue, len(columns))
	for i, col := range columns {
		values[i] = &simpleValue{raw: row[col]}
	}
	return repository.SQLRow{Values: values}
}

// simpleValue implements repository.SQLValue for the in-memory client.
type simpleValue struct {
	raw    interface{}
	intVal int64
}

func (v *simpleValue) GetString() string {
	if v.raw == nil {
		return ""
	}
	return fmt.Sprintf("%v", v.raw)
}

func (v *simpleValue) GetInt() int64 {
	if v.raw != nil {
		switch val := v.raw.(type) {
		case int64:
			return val
		case int32:
			return int64(val)
		case int:
			return int64(val)
		}
	}
	return v.intVal
}

func (v *simpleValue) GetBool() bool {
	if v.raw == nil {
		return false
	}
	b, ok := v.raw.(bool)
	return ok && b
}
