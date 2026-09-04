package repository

import (
	"context"
)

type fakeSQLValue struct {
	stringValue string
	intValue    int64
	boolValue   bool
}

func (value fakeSQLValue) GetString() string { return value.stringValue }
func (value fakeSQLValue) GetInt() int64     { return value.intValue }
func (value fakeSQLValue) GetBool() bool     { return value.boolValue }

type fakeImmudbClient struct {
	query  string
	params map[string]interface{}
	result *SQLResult
}

func (client *fakeImmudbClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	return &SQLResult{}, nil
}

func (client *fakeImmudbClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*SQLResult, error) {
	client.query = sql
	client.params = params
	return client.result, nil
}
