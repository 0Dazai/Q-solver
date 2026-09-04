package interview

import "testing"

func TestNewSessionIDIsNonEmptyAndUnique(t *testing.T) {
	first := newSessionID()
	second := newSessionID()
	if first == "" || second == "" {
		t.Fatal("session IDs must not be empty")
	}
	if first == second {
		t.Fatalf("session IDs must be unique: %q", first)
	}
}
