package providers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func TestTagsProvider_Name(t *testing.T) {
	p := NewTagsProvider(nil)
	assert.Equal(t, "tags", p.Name())
}

func TestTagsProvider_Headers(t *testing.T) {
	p := NewTagsProvider(nil)
	expected := []string{"id", "name", "is_default", "created_at"}
	assert.Equal(t, expected, p.Headers())
}

func TestTagsProvider_Collect_Success(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Tags: []*financepb.TagData{
			{
				Id:        "tag-1",
				Name:      "Food",
				IsDefault: true,
				CreatedAt: "2025-06-01T10:00:00Z",
			},
			{
				Id:        "tag-2",
				Name:      "Transport",
				IsDefault: false,
				CreatedAt: "2025-07-15T14:30:00Z",
			},
		},
	}

	p := NewTagsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, []string{"tag-1", "Food", "true", "2025-06-01T10:00:00Z"}, rows[0])
	assert.Equal(t, []string{"tag-2", "Transport", "false", "2025-07-15T14:30:00Z"}, rows[1])
}

func TestTagsProvider_Collect_EmptyData(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Tags: []*financepb.TagData{},
	}

	p := NewTagsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestTagsProvider_Collect_NilTags(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Tags: nil,
	}

	p := NewTagsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestTagsProvider_Collect_BoolFormatting(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Tags: []*financepb.TagData{
			{Id: "t1", Name: "Default Tag", IsDefault: true, CreatedAt: "2025-01-01T00:00:00Z"},
			{Id: "t2", Name: "Custom Tag", IsDefault: false, CreatedAt: "2025-01-02T00:00:00Z"},
		},
	}

	p := NewTagsProvider(data)
	rows, err := p.Collect(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "true", rows[0][2])
	assert.Equal(t, "false", rows[1][2])
}
