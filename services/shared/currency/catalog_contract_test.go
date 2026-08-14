package currency

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

type catalogEntry struct {
	Code            string `json:"code"`
	MinorUnitDigits int    `json:"minorUnitDigits"`
}

func TestGeneratedCatalogMatchesSource(t *testing.T) {
	entries := loadSourceCatalog(t)

	generatedCodes := make([]string, 0, len(SupportedCurrencies))
	for _, definition := range SupportedCurrencies {
		generatedCodes = append(generatedCodes, definition.Code)
	}

	sourceCodes := make([]string, 0, len(entries))
	for _, entry := range entries {
		sourceCodes = append(sourceCodes, entry.Code)
	}

	sort.Strings(generatedCodes)
	sort.Strings(sourceCodes)

	if len(generatedCodes) != len(sourceCodes) {
		t.Fatalf("generated code set has %d entries, source has %d", len(generatedCodes), len(sourceCodes))
	}
	for i := range sourceCodes {
		if generatedCodes[i] != sourceCodes[i] {
			t.Fatalf("generated code set = %v, source code set = %v", generatedCodes, sourceCodes)
		}
	}

	for _, entry := range entries {
		definition, ok := Get(entry.Code)
		if !ok {
			t.Fatalf("generated catalog missing %s", entry.Code)
		}
		if definition.MinorUnitDigits != entry.MinorUnitDigits {
			t.Fatalf("%s minorUnitDigits = %d, want %d", entry.Code, definition.MinorUnitDigits, entry.MinorUnitDigits)
		}
	}
}

func loadSourceCatalog(t *testing.T) []catalogEntry {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating test file")
	}

	catalogPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "shared", "currency", "catalog.json")
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("reading catalog source: %v", err)
	}

	var entries []catalogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("parsing catalog source: %v", err)
	}
	return entries
}
