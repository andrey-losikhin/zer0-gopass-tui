package gopass

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}
	if len(id) != idEncodedLength {
		t.Fatalf("GenerateID() len = %d, want %d", len(id), idEncodedLength)
	}
	if !ValidCanonicalID(id) {
		t.Fatalf("ValidCanonicalID(%q) = false, want true for generated id", id)
	}
}

func TestGenerateID_Unique(t *testing.T) {
	const n = 100
	seen := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		id, err := GenerateID()
		if err != nil {
			t.Fatalf("GenerateID() error = %v", err)
		}
		if seen[id] {
			t.Fatalf("GenerateID() produced duplicate id %q on iteration %d", id, i)
		}
		seen[id] = true
	}
}

func TestValidCanonicalID(t *testing.T) {
	valid, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}

	// заменяем последний символ валидного id на символ вне urlsafe-алфавита,
	// чтобы получить строку той же длины (22), но с недопустимым символом.
	paddingVariant := valid[:len(valid)-1] + "="
	stdBase64PlusVariant := valid[:len(valid)-1] + "+"
	stdBase64SlashVariant := valid[:len(valid)-1] + "/"

	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"valid canonical id", valid, true},
		{"empty string", "", false},
		{"too short (21 chars)", strings.Repeat("a", 21), false},
		{"too long (23 chars)", strings.Repeat("a", 23), false},
		{"with padding =", paddingVariant, false},
		{"standard base64 + char", stdBase64PlusVariant, false},
		{"standard base64 / char", stdBase64SlashVariant, false},
		{"garbage ASCII, correct length", strings.Repeat("!", 22), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidCanonicalID(tt.id); got != tt.want {
				t.Errorf("ValidCanonicalID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}
