package skills

import "testing"

func TestEmptyRegistryHasNoSkills(t *testing.T) {
	r := NewRegistry()
	if got := r.List(); len(got) != 0 {
		t.Fatalf("expected empty list, got %d skills", len(got))
	}
}

func TestRegistryGetReturnsFalseForUnknown(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("nope"); ok {
		t.Fatal("expected Get to return false for unknown name")
	}
}
