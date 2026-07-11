package providers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ItsThompson/gofin/services/finance/proto/financepb"
)

func TestBuildTagMap_MapsIDToName(t *testing.T) {
	data := &financepb.AllUserDataResponse{
		Tags: []*financepb.TagData{
			{Id: "tag-1", Name: "Food"},
			{Id: "tag-2", Name: "Transport"},
		},
	}

	got := BuildTagMap(data)

	assert.Equal(t, map[string]string{"tag-1": "Food", "tag-2": "Transport"}, got)
}

func TestBuildTagMap_EmptyTags(t *testing.T) {
	got := BuildTagMap(&financepb.AllUserDataResponse{Tags: []*financepb.TagData{}})
	assert.Empty(t, got)
}

func TestBuildTagMap_NilResponseIsSafe(t *testing.T) {
	// Protobuf getters are nil-safe; a nil response yields an empty map.
	got := BuildTagMap(nil)
	assert.Empty(t, got)
}
