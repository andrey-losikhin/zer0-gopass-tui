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
	if c.mode != cardEditAll || len(c.editor.fields) != 2 || c.editor.fields[0].Name != "Username" {
		t.Fatalf("editor with existing fields not opened: mode=%v fields=%d", c.mode, len(c.editor.fields))
	}
	c.editor.cursor = 1 // Username; row 0 is the locked path.
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c.editor.input.SetValue("bob")
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, cmd, _ := c.updateKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil {
		t.Fatal("edit returned nil command")
	}
	_ = cmd()
	if len(w.replaced) != 2 || w.replaced[0].Name != "Username" || w.replaced[0].Value != "bob" {
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
	c.editor.cursor = 1
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c.editor.input.SetValue("draft")
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, cmd, _ := c.updateKey(tea.KeyMsg{Type: tea.KeyCtrlS})
	msg := cmd().(createdMsg)
	if msg.err != gopass.ErrStaleRevision || c.editor.fields[0].Value != "draft" {
		t.Fatalf("error=%v draft=%q", msg.err, c.editor.fields[0].Value)
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

func TestCardRenameMovesEntryAndRelinksBitwarden(t *testing.T) {
	m, _, w := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	syncer := &fakeBitwardenSyncer{}
	m.bitwarden = syncer
	m.mode = modeCard
	m.card = readyCard(w)
	m.card.set.BitwardenSync = true
	updated, _ := m.Update(keyRunes("R"))
	m = updated.(Model)
	if m.card.mode != cardRename || !m.acceptingText() {
		t.Fatalf("rename input not opened: mode=%v", m.card.mode)
	}
	m.card.rename.SetValue("work/renamed")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, cmd = m.Update(cmd())
	m = updated.(Model)
	if w.movedFrom != "work/account" || w.movedTo != "work/renamed" || m.card.entry != "work/renamed" {
		t.Fatalf("moved %q -> %q, card=%q", w.movedFrom, w.movedTo, m.card.entry)
	}
	for _, msg := range drainBatch(cmd) {
		if _, ok := msg.(bitwardenSyncedMsg); ok {
			if len(syncer.relinked) != 1 {
				t.Fatalf("relinked=%v", syncer.relinked)
			}
			return
		}
	}
	t.Fatal("Bitwarden relink was not scheduled")
}

func TestCardCloneCopiesOnlyPublicValues(t *testing.T) {
	m, r, w := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	m.mode = modeCard
	m.card = readyCard(w)
	updated, _ := m.Update(keyRunes("c"))
	m = updated.(Model)
	if m.mode != modeCreate || m.create.locked != "" || m.create.path.Value() != "work/account-copy" {
		t.Fatalf("clone form mode=%v path=%q", m.mode, m.create.path.Value())
	}
	if len(m.create.fields) != 2 || m.create.fields[0].Value != "alice" || m.create.fields[1].Value != "" || r.resolves != 0 {
		t.Fatalf("clone fields = %#v resolves=%d", m.create.fields, r.resolves)
	}
}

func TestRenameInputAcceptsNavigationLetters(t *testing.T) {
	m, _, w := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	m.mode = modeCard
	m.card = readyCard(w)
	updated, _ := m.Update(keyRunes("R"))
	m = updated.(Model)
	for _, key := range []string{"h", "q"} {
		updated, _ = m.Update(keyRunes(key))
		m = updated.(Model)
	}
	if m.mode != modeCard || m.quitting || m.card.rename.Value() != "work/accounthq" {
		t.Fatalf("mode=%v quitting=%v value=%q", m.mode, m.quitting, m.card.rename.Value())
	}
}
