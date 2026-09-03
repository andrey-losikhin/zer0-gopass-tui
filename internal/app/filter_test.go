package app

import (
	"reflect"
	"testing"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestFilterEntries(t *testing.T) {
	entries := []gopass.Entry{
		{Path: "web/example.com"},
		{Path: "email/gmail"},
		{Path: "banking/example-bank"},
	}

	tests := []struct {
		name  string
		query string
		want  []gopass.Entry
	}{
		{
			name:  "empty query returns all entries unchanged",
			query: "",
			want:  entries,
		},
		{
			name:  "query matches subset by fuzzy subsequence",
			query: "exam",
			want: []gopass.Entry{
				{Path: "web/example.com"},
				{Path: "banking/example-bank"},
			},
		},
		{
			name:  "query matches nothing",
			query: "zzzz",
			want:  []gopass.Entry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEntries(entries, tt.query)
			if len(got) != len(tt.want) {
				t.Fatalf("filterEntries(%q) len = %d, want %d (got %v)", tt.query, len(got), len(tt.want), got)
			}
			if len(tt.want) > 0 && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("filterEntries(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

func TestFuzzyMatch(t *testing.T) {
	tests := []struct {
		name  string
		query string
		path  string
		want  bool
	}{
		{name: "exact match", query: "foo", path: "foo", want: true},
		{name: "subsequence match", query: "fb", path: "foo/bar", want: true},
		{name: "case insensitive match", query: "FOO", path: "foo", want: true},
		{name: "order matters, not a subsequence", query: "ba", path: "ab", want: false},
		{name: "query longer than path", query: "foobar", path: "foo", want: false},
		{name: "empty query always matches", query: "", path: "foo", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fuzzyMatch(tt.query, tt.path); got != tt.want {
				t.Errorf("fuzzyMatch(%q, %q) = %v, want %v", tt.query, tt.path, got, tt.want)
			}
		})
	}
}

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		length int
		want   int
	}{
		{name: "empty list always clamps to 0", cursor: 5, length: 0, want: 0},
		{name: "negative cursor clamps to 0", cursor: -1, length: 3, want: 0},
		{name: "cursor beyond length clamps to last index", cursor: 10, length: 3, want: 2},
		{name: "cursor within bounds unchanged", cursor: 1, length: 3, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampCursor(tt.cursor, tt.length); got != tt.want {
				t.Errorf("clampCursor(%d, %d) = %d, want %d", tt.cursor, tt.length, got, tt.want)
			}
		})
	}
}
