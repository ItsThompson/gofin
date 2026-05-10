package providers

import (
	"context"
	"fmt"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that TagsProvider implements DataProvider.
var _ engine.DataProvider = (*TagsProvider)(nil)

// TagsProvider fetches user tags from the finance service.
type TagsProvider struct {
	financeClient financepb.FinanceServiceClient
}

// NewTagsProvider creates a TagsProvider backed by the finance gRPC client.
func NewTagsProvider(financeClient financepb.FinanceServiceClient) *TagsProvider {
	return &TagsProvider{financeClient: financeClient}
}

// Name returns the CSV filename for this provider.
func (p *TagsProvider) Name() string {
	return "tags"
}

// Headers returns the CSV column headers for tag data.
func (p *TagsProvider) Headers() []string {
	return []string{"id", "name", "is_default", "created_at"}
}

// Collect fetches all tags for the user and returns formatted rows.
func (p *TagsProvider) Collect(ctx context.Context, userID string) ([][]string, error) {
	resp, err := p.financeClient.GetAllUserData(ctx, &financepb.GetAllUserDataRequest{UserId: userID})
	if err != nil {
		return nil, fmt.Errorf("fetching user data for tags: %w", err)
	}

	tags := resp.GetTags()
	rows := make([][]string, 0, len(tags))
	for _, tag := range tags {
		rows = append(rows, []string{
			tag.GetId(),
			tag.GetName(),
			formatBool(tag.GetIsDefault()),
			tag.GetCreatedAt(),
		})
	}

	return rows, nil
}
