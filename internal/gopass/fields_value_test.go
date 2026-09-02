package gopass

import (
	"bytes"
	"testing"
)

func TestValidFieldValue(t *testing.T) {
	tests := []struct {
		name      string
		value     []byte
		multiline bool
		wantErr   bool
	}{
		{"valid single-line value", []byte("hello"), false, false},
		{"valid multiline value with LF", []byte("line1\nline2"), true, false},
		{"single-line value with LF", []byte("a\nb"), false, true},
		{"CR present even when multiline", []byte("a\rb"), true, true},
		{"NUL byte", []byte("a\x00b"), false, true},
		{"empty value", []byte(""), false, true},
		{"value larger than 1 MiB", bytes.Repeat([]byte("a"), 1024*1024+1), false, true},
		{"invalid UTF-8", []byte{0xff, 0xfe}, false, true},
		{"multiline value with TAB", []byte("a\tb"), true, false},
		{"single-line value with TAB", []byte("a\tb"), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidFieldValue(tt.value, tt.multiline)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidFieldValue(_, multiline=%v) error = %v, wantErr %v", tt.multiline, err, tt.wantErr)
			}
		})
	}
}
