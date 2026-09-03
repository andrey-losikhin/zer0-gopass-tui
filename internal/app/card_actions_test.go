package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func readyCard(writer *fakeWriter) cardModel {
	c := newCard(context.Background(), &fakeReader{revealed: "secret"}, writer, "work/account")
	c.loading = false
	c.set = testSet()
	return c
}

func TestCardEditsSelectedField(t *testing.T) {
	w := &fakeWriter{set: testSet()}
	c := readyCard(w)
	c, load, _ := c.updateKey(keyRunes("e"))
	c, _, _ = c.update(load())
	if c.mode != cardEditAll || len(c.editor.fields) != 20 || c.editor.fields[1].Name != "Username" {
		t.Fatalf("full editor not opened: %#v", c)
	}
	c.editor.cursor = 2 // Username; row 0 is the locked path.
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c.editor.input.SetValue("bob")
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, cmd, _ := c.updateKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("edit returned nil command")
	}
	_ = cmd()
	if len(w.replaced) != 2 || w.replaced[1].Name != "Username" || w.replaced[1].Value != "bob" {
		t.Fatalf("replaced fields=%#v", w.replaced)
	}
}

func TestCardAddsStandardField(t *testing.T) {
	w := &fakeWriter{set: testSet()}
	c := readyCard(w)
	c, _, _ = c.updateKey(keyRunes("a"))
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyDown})
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyDown}) // url
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter}) // accept URL name
	c.form.input.SetValue("https://example.test")
	c, cmd, _ := c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, _, _ = c.update(cmd())
	if !w.added || c.adding {
		t.Fatalf("added=%v card.adding=%v", w.added, c.adding)
	}
}

func TestCardDeletesFieldOnlyAfterConfirmation(t *testing.T) {
	w := &fakeWriter{set: testSet()}
	c := readyCard(w)
	c, _, _ = c.updateKey(keyRunes("d"))
	if c.mode != cardConfirmField || w.deletedField != "" {
		t.Fatal("field delete skipped confirmation")
	}
	c, cmd, _ := c.updateKey(keyRunes("y"))
	c, _, _ = c.update(cmd())
	if w.deletedField != "user-id" {
		t.Fatalf("deleted field = %q", w.deletedField)
	}
}

func TestStaleEditKeepsDraft(t *testing.T) {
	w := &fakeWriter{err: gopass.ErrStaleRevision}
	c := readyCard(w)
	c, load, _ := c.updateKey(keyRunes("e"))
	c, _, _ = c.update(load())
	c.editor.cursor = 2
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c.editor.input.SetValue("draft")
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, cmd, _ := c.updateKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	msg := cmd().(createdMsg)
	if msg.err != gopass.ErrStaleRevision || c.editor.fields[1].Value != "draft" {
		t.Fatalf("error=%v draft=%q", msg.err, c.editor.fields[1].Value)
	}
}

func TestStaleAddKeepsDraft(t *testing.T) {
	w := &fakeWriter{err: gopass.ErrStaleRevision}
	c := readyCard(w)
	c.mode = cardEdit
	c.adding = true
	field, _ := gopass.StandardField("url")
	c.form = newValueForm(field, false)
	c.form.input.SetValue("https://draft.test")
	c, cmd, _ := c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, _, _ = c.update(cmd())
	if c.mode != cardEdit || !c.adding || c.form.field.Value != "https://draft.test" {
		t.Fatalf("mode=%v adding=%v draft=%q", c.mode, c.adding, c.form.field.Value)
	}
}

func TestBackendReadErrorIsNotLegacy(t *testing.T) {
	c := newCard(context.Background(), &fakeReader{}, &fakeWriter{}, "work/account")
	c, _, _ = c.update(fieldsLoadedMsg{entry: "work/account", err: context.DeadlineExceeded})
	if c.legacy || !c.fatal {
		t.Fatalf("legacy=%v fatal=%v", c.legacy, c.fatal)
	}
}
