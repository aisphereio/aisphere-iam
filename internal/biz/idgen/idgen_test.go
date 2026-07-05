package idgen

import (
	"regexp"
	"testing"
)

func TestNewUsesSpiceDBSafeCharacters(t *testing.T) {
	got := New("org")
	if !regexp.MustCompile(`^org_[a-zA-Z0-9/_|\-=+]+$`).MatchString(got) {
		t.Fatalf("New returned SpiceDB-unsafe ID %q", got)
	}
}
