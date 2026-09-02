package gopass

import (
	"reflect"
	"testing"
)

func TestParseListOutput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []Entry
	}{
		{
			name:  "empty output",
			input: "",
			want:  []Entry{},
		},
		{
			name:  "space-only paths are preserved, tab path is rejected",
			input: "   \r\n\t\r\n   \n",
			want:  []Entry{{Path: "   "}, {Path: "   "}},
		},
		{
			name:  "regular paths with trailing newline",
			input: "foo\nbar/baz\nqux\n",
			want: []Entry{
				{Path: "foo"},
				{Path: "bar/baz"},
				{Path: "qux"},
			},
		},
		{
			name:  "no trailing newline on last line",
			input: "foo\nbar/baz\nqux",
			want: []Entry{
				{Path: "foo"},
				{Path: "bar/baz"},
				{Path: "qux"},
			},
		},
		{
			name:  "empty lines between entries are skipped, no empty Entry created",
			input: "foo\n\n\nbar\n",
			want: []Entry{
				{Path: "foo"},
				{Path: "bar"},
			},
		},
		{
			name:  "CRLF line endings",
			input: "foo\r\nbar/baz\r\n",
			want: []Entry{
				{Path: "foo"},
				{Path: "bar/baz"},
			},
		},
		{
			name:  "leading and trailing spaces are significant, controls rejected",
			input: "  foo  \n\tbar/baz\t\n",
			want: []Entry{
				{Path: "  foo  "},
			},
		},
		{
			name:  "control and bidi paths are rejected",
			input: "safe\nevil\x1b[31m\nabc\u202edef\n",
			want:  []Entry{{Path: "safe"}},
		},
		{
			name:  "reserved namespace is excluded",
			input: "user/account\n.zer0-waypass/v1/manifests/id\n.zer0-waypass/v1/bundle/revision/field\n",
			want:  []Entry{{Path: "user/account"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListOutput(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseListOutput(%q) len = %d, want %d (got %v)", tt.input, len(got), len(tt.want), got)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseListOutput(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
