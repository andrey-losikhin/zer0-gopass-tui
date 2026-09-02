package app

import "testing"

func TestCommandKeySupportsRussianLayout(t *testing.T) {
	tests := map[string]string{
		"й": "q", "т": "n", "в": "d", "у": "e", "ф": "a",
		"к": "r", "ч": "x", "о": "j", "л": "k", "н": "y",
		"м": "v", "ь": "m", "п": "g", "и": "b", ".": "/",
	}
	for input, want := range tests {
		if got := commandKey(keyRunes(input)); got != want {
			t.Errorf("commandKey(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRussianCreateShortcutAndPathInput(t *testing.T) {
	m, _, _ := loadedModel(t, nil)
	updated, _ := m.Update(keyRunes("т"))
	m = updated.(Model)
	if m.mode != modeCreate {
		t.Fatalf("mode = %v, want create", m.mode)
	}
	updated, _ = m.Update(keyRunes("аккаунт"))
	m = updated.(Model)
	if m.create.input.Value() != "аккаунт" {
		t.Fatalf("path input = %q", m.create.input.Value())
	}
}
