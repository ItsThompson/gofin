package errkit

import "errors"

// DataCarrier is implemented by errors that carry structured report data.
// report() merges ReportData() into Meta.Data before logging and capturing,
// so a typed error contributes context without the call site repeating it.
type DataCarrier interface {
	ReportData() map[string]any
}

// mergeData returns the report data for err merged under caller. Caller wins on
// key collision. The caller's map is never mutated: when err carries data a new
// map is built, otherwise caller is returned unchanged so a non-carrier error
// keeps the existing behavior.
func mergeData(caller map[string]any, err error) map[string]any {
	var carrier DataCarrier
	if !errors.As(err, &carrier) {
		return caller
	}

	carried := carrier.ReportData()
	if len(carried) == 0 {
		return caller
	}

	merged := make(map[string]any, len(carried)+len(caller))
	for key, value := range carried {
		merged[key] = value
	}
	for key, value := range caller {
		merged[key] = value
	}
	return merged
}
