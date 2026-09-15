package collector

import (
	"context"
	"errors"
	"fmt"
	"sort"

	reportingpb "github.com/ItsThompson/gofin/services/shared/reporting/proto/reportingpb"
	"google.golang.org/protobuf/proto"
)

type indexedResult struct {
	index  int
	result GroupResult
}

func (c Collector) Collect(ctx context.Context, windows *reportingpb.ReportWindowSet) Collection {
	results := make(chan indexedResult, len(c.Clients))
	groupNames := make([]string, len(c.Clients))
	for index, client := range c.Clients {
		groupNames[index] = safeGroupName(client, index)
	}
	for index, client := range c.Clients {
		groupName := groupNames[index]
		go func() {
			requestContext := ctx
			cancel := func() {}
			if c.Timeout > 0 {
				requestContext, cancel = context.WithTimeout(ctx, c.Timeout)
			}
			defer cancel()
			result := collectSafely(client, groupName, requestContext, windows)
			results <- indexedResult{index: index, result: result}
		}()
	}

	groups := make([]GroupResult, len(c.Clients))
	completed := make([]bool, len(c.Clients))
	remaining := len(c.Clients)
	for remaining > 0 {
		select {
		case item := <-results:
			if completed[item.index] {
				continue
			}
			groups[item.index] = item.result
			completed[item.index] = true
			remaining--
		case <-ctx.Done():
		drain:
			for {
				select {
				case item := <-results:
					if completed[item.index] {
						continue
					}
					groups[item.index] = item.result
					completed[item.index] = true
					remaining--
				default:
					for index := range c.Clients {
						if !completed[index] {
							groups[index] = failedGroup(groupNames[index], ctx.Err())
						}
					}
					remaining = 0
					break drain
				}
			}
		}
	}
	collection := Collection{Groups: groups}
	sort.SliceStable(collection.Groups, func(i, j int) bool {
		return collection.Groups[i].Name < collection.Groups[j].Name
	})
	return collection
}

func collectSafely(client GroupClient, name string, ctx context.Context, windows *reportingpb.ReportWindowSet) (result GroupResult) {
	defer func() {
		if recover() != nil {
			result = failedGroup(name, errors.New("client panic"))
		}
	}()
	result = client.Collect(ctx, cloneWindowSet(windows))
	if result.Name == "" {
		result.Name = name
	}
	return result
}

func safeGroupName(client GroupClient, index int) (name string) {
	name = fmt.Sprintf("group-%d", index)
	defer func() {
		if recover() != nil || name == "" {
			name = fmt.Sprintf("group-%d", index)
		}
	}()
	if client != nil {
		if candidate := client.Name(); candidate != "" {
			name = candidate
		}
	}
	return name
}

func cloneWindowSet(windows *reportingpb.ReportWindowSet) *reportingpb.ReportWindowSet {
	if windows == nil {
		return nil
	}
	return proto.Clone(windows).(*reportingpb.ReportWindowSet)
}
