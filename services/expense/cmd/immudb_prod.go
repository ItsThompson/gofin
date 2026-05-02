// This file provides the real immudb client for production/Docker builds.
// It requires the github.com/codenotary/immudb dependency, which is fetched
// during `go mod download` in the Docker build stage.

//go:build docker

package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	immudb "github.com/codenotary/immudb/pkg/client"

	"github.com/ItsThompson/gofin/services/expense/internal/config"
	"github.com/ItsThompson/gofin/services/expense/internal/repository"
)

// realImmudbClient wraps the immudb native Go client.
type realImmudbClient struct {
	client immudb.ImmuClient
}

func newImmudbClientImpl(ctx context.Context, cfg *config.Config) (repository.ImmudbClient, error) {
	// Parse host:port from IMMUDB_ADDR
	parts := strings.SplitN(cfg.ImmudbAddr, ":", 2)
	host := parts[0]
	port := 3322
	if len(parts) == 2 {
		parsed, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parsing immudb port: %w", err)
		}
		port = parsed
	}

	opts := immudb.DefaultOptions().WithAddress(host).WithPort(port)
	client := immudb.NewClient().WithOptions(opts)

	err := client.OpenSession(ctx, []byte(cfg.ImmudbUsername), []byte(cfg.ImmudbPassword), "defaultdb")
	if err != nil {
		return nil, fmt.Errorf("opening immudb session: %w", err)
	}

	return &realImmudbClient{client: client}, nil
}

func (c *realImmudbClient) SQLExec(ctx context.Context, sql string, params map[string]interface{}) (*repository.SQLResult, error) {
	_, err := c.client.SQLExec(ctx, sql, params)
	if err != nil {
		return nil, err
	}
	return &repository.SQLResult{}, nil
}

func (c *realImmudbClient) SQLQuery(ctx context.Context, sql string, params map[string]interface{}) (*repository.SQLResult, error) {
	result, err := c.client.SQLQuery(ctx, sql, params, true)
	if err != nil {
		return nil, err
	}

	rows := make([]repository.SQLRow, len(result.Rows))
	for i, row := range result.Rows {
		values := make([]repository.SQLValue, len(row.Values))
		for j, val := range row.Values {
			values[j] = &immudbValue{val: val}
		}
		rows[i] = repository.SQLRow{Values: values}
	}

	return &repository.SQLResult{Rows: rows}, nil
}

// immudbValue wraps an immudb SQL value to satisfy repository.SQLValue.
type immudbValue struct {
	val interface{ GetS() string; GetN() int64; GetB() bool }
}

func (v *immudbValue) GetString() string { return v.val.GetS() }
func (v *immudbValue) GetInt() int64     { return v.val.GetN() }
func (v *immudbValue) GetBool() bool     { return v.val.GetB() }
