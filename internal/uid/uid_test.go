package uid

import "testing"

func TestNew_Length(t *testing.T) {
	id := New()
	if len(id) != 16 {
		t.Errorf("expected 16 hex chars, got %d: %s", len(id), id)
	}
}

func TestNew_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := New()
		if seen[id] {
			t.Fatalf("duplicate ID: %s", id)
		}
		seen[id] = true
	}
}

func TestNew_HexChars(t *testing.T) {
	id := New()
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("non-hex char %c in ID %s", c, id)
		}
	}
}
