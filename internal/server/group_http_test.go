package server

import (
	"strings"
	"testing"
)

func TestSlugifyGroupName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"plain ascii", "platform", "platform"},
		{"mixed case", "Platform", "platform"},
		{"spaces collapse", "my platform", "my-platform"},
		{"leading trailing spaces", "  platform  ", "platform"},
		{"underscores become hyphens", "my_platform", "my-platform"},
		{"special chars", "dev/team:ops", "dev-team-ops"},
		{"dots removed", "v2.0.team", "v2-0-team"},
		{"multiple hyphens collapse", "a---b", "a-b"},
		{"leading trailing hyphens trimmed", "-platform-", "platform"},
		{"numbers preserved", "team42", "team42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slugifyGroupName(tt.raw)
			if got != tt.want {
				t.Errorf("slugifyGroupName(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestSlugifyGroupNameNonASCII(t *testing.T) {
	// Chinese / non-ASCII input should fall back to a "group-" prefixed
	// random string because no [a-z0-9-] characters survive.
	for _, raw := range []string{"研发部", "组織", "プラットフォーム", "🎉", "   ", ""} {
		got := slugifyGroupName(raw)
		if !strings.HasPrefix(got, "group-") {
			t.Errorf("slugifyGroupName(%q) = %q, expected \"group-\" prefix for non-ASCII/empty input", raw, got)
		}
		if len(got) <= len("group-") {
			t.Errorf("slugifyGroupName(%q) = %q, expected a non-empty random suffix", raw, got)
		}
	}
}

func TestSlugifyGroupNameLengthCap(t *testing.T) {
	long := strings.Repeat("a", 200)
	got := slugifyGroupName(long)
	if len(got) > 80 {
		t.Errorf("slugifyGroupName length = %d, want <= 80", len(got))
	}
}

func TestGenerateUniqueGroupName_NoCollision(t *testing.T) {
	existing := map[string]bool{}
	got := generateUniqueGroupName("platform", existing, "")
	if got != "platform" {
		t.Errorf("got %q, want %q", got, "platform")
	}
}

func TestGenerateUniqueGroupName_Collision(t *testing.T) {
	existing := map[string]bool{
		"platform": true,
	}
	got := generateUniqueGroupName("platform", existing, "")
	if got != "platform-2" {
		t.Errorf("got %q, want %q", got, "platform-2")
	}
}

func TestGenerateUniqueGroupName_MultipleCollisions(t *testing.T) {
	existing := map[string]bool{
		"platform":   true,
		"platform-2": true,
		"platform-3": true,
	}
	got := generateUniqueGroupName("platform", existing, "")
	if got != "platform-4" {
		t.Errorf("got %q, want %q", got, "platform-4")
	}
}

func TestGenerateUniqueGroupName_SkipSelf(t *testing.T) {
	// On the update path, the group's own current name must not count as a
	// collision so it can keep its name unchanged.
	existing := map[string]bool{
		"platform":   true,
		"platform-2": true,
	}
	got := generateUniqueGroupName("platform", existing, "platform")
	if got != "platform" {
		t.Errorf("got %q, want %q (skipName should exclude self)", got, "platform")
	}
}

func TestGenerateUniqueGroupName_SlugifiesInput(t *testing.T) {
	existing := map[string]bool{}
	got := generateUniqueGroupName("My Platform", existing, "")
	if got != "my-platform" {
		t.Errorf("got %q, want %q", got, "my-platform")
	}
}

func TestGenerateUniqueGroupName_NonASCII(t *testing.T) {
	existing := map[string]bool{}
	got := generateUniqueGroupName("研发部", existing, "")
	if !strings.HasPrefix(got, "group-") {
		t.Errorf("got %q, expected \"group-\" prefix for non-ASCII input", got)
	}
	// Calling again should still produce a unique value (different random suffix).
	got2 := generateUniqueGroupName("研发部", map[string]bool{strings.ToLower(got): true}, "")
	if strings.EqualFold(got, got2) {
		t.Errorf("expected different random suffixes, got %q twice", got)
	}
}
