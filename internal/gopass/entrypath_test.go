package gopass

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncodeDecodeCanonicalPath_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"simple two segments", "github/account"},
		{"segment with spaces", "with spaces/name"},
		{"unicode segment", "юникод/имя"},
		{"nested segments", "a/b/c/d"},
		{"dotfile segment", ".ssh/config"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeCanonicalPath(tt.path)
			if err != nil {
				t.Fatalf("EncodeCanonicalPath(%q) error = %v", tt.path, err)
			}
			decoded, err := DecodeEntryID(encoded)
			if err != nil {
				t.Fatalf("DecodeEntryID(%q) error = %v", encoded, err)
			}
			if decoded != tt.path {
				t.Fatalf("round-trip = %q, want %q", decoded, tt.path)
			}

			// повторное кодирование декодированного значения должно давать тот же EntryID
			// (детерминированное кодирование, отсутствие скрытого состояния).
			reencoded, err := EncodeCanonicalPath(decoded)
			if err != nil {
				t.Fatalf("EncodeCanonicalPath(%q) error = %v", decoded, err)
			}
			if reencoded != encoded {
				t.Fatalf("re-encoded = %q, want %q", reencoded, encoded)
			}
		})
	}
}

func TestEncodeCanonicalPath_Rejects(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"empty string", ""},
		{"absolute path", "/foo"},
		{"leading slash", "/a"},
		{"parent traversal segment", "a/../b"},
		{"dot segment", "a/./b"},
		{"empty segment (double slash)", "a//b"},
		{"trailing slash (empty segment)", "a/"},
		{"segment starting with dash", "a/-flag"},
		{"backslash", "a\\b"},
		{"control character", "a\x01b"},
		{"too long", strings.Repeat("a", maxCanonicalPathBytes+1)},
		{"shell metachar semicolon", "a;b"},
		{"shell metachar pipe", "a|b"},
		{"shell metachar dollar", "a$b"},
		{"bidi RLO control character", "a\u202eb"},
		{"bidi LRM control character", "a\u200eb"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeCanonicalPath(tt.path)
			if err == nil {
				t.Fatalf("EncodeCanonicalPath(%q) error = nil, want error", tt.path)
			}
			if got != "" {
				t.Fatalf("EncodeCanonicalPath(%q) = %q, want empty on error", tt.path, got)
			}
		})
	}
}

func TestDecodeEntryID_Rejects(t *testing.T) {
	// base64url-валидная строка с неверным version byte (0x02 вместо 0x01).
	wrongVersion := base64.RawURLEncoding.EncodeToString(append([]byte{0x02}, "foo/bar"...))
	// декодированная последовательность длины 0 (пустая исходная строка).
	zeroLength := base64.RawURLEncoding.EncodeToString([]byte{})

	tests := []struct {
		name    string
		entryID string
	}{
		{"invalid base64 garbage", "!!!not-valid-base64???"},
		{"wrong version byte", wrongVersion},
		{"zero-length decoded payload", zeroLength},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeEntryID(tt.entryID)
			if err == nil {
				t.Fatalf("DecodeEntryID(%q) error = nil, want error", tt.entryID)
			}
			if got != "" {
				t.Fatalf("DecodeEntryID(%q) = %q, want empty on error", tt.entryID, got)
			}
		})
	}
}
