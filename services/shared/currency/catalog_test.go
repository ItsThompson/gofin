package currency

import "testing"

func TestCatalogCodesAreUnique(t *testing.T) {
	definitions := All()
	seen := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		if seen[definition.Code] {
			t.Fatalf("duplicate currency code %s", definition.Code)
		}
		seen[definition.Code] = true
	}
}

func TestUSDPresentWithExpectedFields(t *testing.T) {
	definition, ok := Get("USD")
	if !ok {
		t.Fatal("USD missing from catalog")
	}
	if definition.Symbol != "$" {
		t.Fatalf("USD symbol = %q, want $", definition.Symbol)
	}
	if definition.Name != "US Dollar" {
		t.Fatalf("USD name = %q, want US Dollar", definition.Name)
	}
	if definition.MinorUnitDigits != 2 {
		t.Fatalf("USD minorUnitDigits = %d, want 2", definition.MinorUnitDigits)
	}
}

func TestGetUnknownCodeReturnsFalse(t *testing.T) {
	if _, ok := Get("XXX"); ok {
		t.Fatal("Get returned a definition for an unknown code")
	}
}

func TestIsSupported(t *testing.T) {
	if !IsSupported("EUR") {
		t.Fatal("IsSupported(EUR) = false, want true")
	}
	if IsSupported("xxx") {
		t.Fatal("IsSupported(xxx) = true, want false")
	}
}

func TestAllReturnsCopy(t *testing.T) {
	definitions := All()
	if len(definitions) == 0 {
		t.Fatal("catalog is empty")
	}

	definitions[0].Code = "XXX"

	fresh := All()
	if fresh[0].Code == "XXX" {
		t.Fatal("All returned mutable package state")
	}
}
