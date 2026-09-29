package app

import (
	"context"
	"strings"
	"testing"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestDuplicateCheckRequiresConfirmationAndReportsOnlyPaths(t *testing.T) {
	vault := newFakeVault()
	password, _ := gopass.StandardField("password")
	for path, value := range map[string]string{"a": "same-synthetic", "b": "same-synthetic", "c": "unique-synthetic"} {
		field := password
		field.Value = value
		if _, err := vault.CreateBundle(context.Background(), path, []gopass.FieldValue{field}); err != nil {
			t.Fatal(err)
		}
	}
	m := NewModel(context.Background(), vault, vault, vault)
	entries, _ := vault.List(context.Background())
	updated, _ := m.Update(entriesLoadedMsg{entries: entries})
	m = updated.(Model)
	m.mode = modeList
	updated, cmd := m.Update(keyRunes("D"))
	m = updated.(Model)
	if cmd != nil || !m.duplicates.confirm || !strings.Contains(m.View(), "расшифрует") {
		t.Fatal("duplicate check started without confirmation")
	}
	updated, cmd = m.Update(keyRunes("y"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	result := m.duplicates.result
	if result == nil || result.checked != 3 || len(result.groups) != 1 || strings.Join(result.groups[0], ",") != "a,b" {
		t.Fatalf("result = %#v", result)
	}
	if view := m.View(); strings.Contains(view, "same-synthetic") || !strings.Contains(view, "a, b") {
		t.Fatalf("view leaked password or missed group: %q", view)
	}
}
