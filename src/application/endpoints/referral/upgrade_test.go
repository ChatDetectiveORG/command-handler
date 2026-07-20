package referral

import "testing"

func TestParseUpgradeLevels(t *testing.T) {
	levels, err := parseUpgradeLevels("/upgrade 4")
	if !err.IsNil() {
		t.Fatalf("unexpected error: %s", err.JSON())
	}
	if levels != 4 {
		t.Fatalf("expected 4 levels, got %d", levels)
	}

	levels, err = parseUpgradeLevels("/upgrade@ChatDetectiveBot 2")
	if !err.IsNil() {
		t.Fatalf("unexpected bot command error: %s", err.JSON())
	}
	if levels != 2 {
		t.Fatalf("expected 2 levels, got %d", levels)
	}
}

func TestParseUpgradeLevelsRejectsInvalidInput(t *testing.T) {
	for _, input := range []string{"/upgrade", "/upgrade 0", "/upgrade -1", "/upgrade many", "/upgrade 1 2"} {
		if _, err := parseUpgradeLevels(input); err.IsNil() {
			t.Fatalf("expected error for %q", input)
		}
	}
}
