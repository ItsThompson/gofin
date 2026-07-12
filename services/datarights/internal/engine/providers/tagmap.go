package providers

import "github.com/ItsThompson/gofin/services/finance/proto/financepb"

// BuildTagMap derives a tag-id -> tag-name lookup from the shared per-job
// finance response. The export fetches GetAllUserData once (in engine.execute);
// this map and the tags/budget_periods/default_settings rows all derive from
// that single response, so the export hits finance exactly once per job. The
// expenses provider consumes the returned map to resolve tag names.
func BuildTagMap(data *financepb.AllUserDataResponse) map[string]string {
	tags := data.GetTags()
	tagMap := make(map[string]string, len(tags))
	for _, tag := range tags {
		tagMap[tag.GetId()] = tag.GetName()
	}
	return tagMap
}
