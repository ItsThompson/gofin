package providers

import (
	"context"

	"github.com/ItsThompson/gofin/services/datarights/internal/engine"
	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

// Compile-time check that TagsProvider implements DataProvider.
var _ engine.DataProvider = (*TagsProvider)(nil)

// TagsProvider maps the tags in the shared per-job finance response into rows.
type TagsProvider struct {
	data *financepb.AllUserDataResponse
}

// NewTagsProvider creates a TagsProvider over the finance data the export engine
// fetches once per job.
func NewTagsProvider(data *financepb.AllUserDataResponse) *TagsProvider {
	return &TagsProvider{data: data}
}

// Name returns the CSV filename for this provider.
func (p *TagsProvider) Name() string {
	return "tags"
}

// Headers returns the CSV column headers for tag data.
func (p *TagsProvider) Headers() []string {
	return []string{"id", "name", "is_default", "created_at"}
}

// Collect maps the pre-fetched user data's tags into rows. It is a pure mapper:
// the finance fetch happens once in the export engine, so Collect issues no RPC.
func (p *TagsProvider) Collect(_ context.Context, _ string) ([][]string, error) {
	tags := p.data.GetTags()
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
