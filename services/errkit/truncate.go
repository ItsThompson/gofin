package errkit

import (
	"strings"
	"unicode/utf8"
)

// maxTagValueRunes is Sentry's per-tag-value limit, counted in characters.
const maxTagValueRunes = 200

// truncateTagValue makes value safe to send as a Sentry tag. A newline becomes a
// space so the value stays one searchable token, matching the frontend helper.
// The cut is rune-based rather than byte-based because Sentry counts characters
// and cutting bytes can split a multi-byte character into invalid UTF-8.
func truncateTagValue(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")

	if utf8.RuneCountInString(value) <= maxTagValueRunes {
		return value
	}
	return string([]rune(value)[:maxTagValueRunes])
}
