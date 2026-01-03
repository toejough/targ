//go:build commander

package issues

import "testing"

func TestNormalizeStatus(t *testing.T) {
	cases := map[string]string{
		"backlog":     "backlog",
		"Done":        "done",
		"in-progress": "in progress",
		"in_progress": "in progress",
		"inprogress":  "in progress",
		" review ":    "review",
	}
	for input, want := range cases {
		if got := normalizeStatus(input); got != want {
			t.Fatalf("normalizeStatus(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizePriority(t *testing.T) {
	cases := map[string]string{
		"low":    "Low",
		"LOW":    "Low",
		"Medium": "Medium",
		"high":   "High",
		"":       "",
		"custom": "custom",
	}
	for input, want := range cases {
		if got := normalizePriority(input); got != want {
			t.Fatalf("normalizePriority(%q) = %q, want %q", input, got, want)
		}
	}
}
