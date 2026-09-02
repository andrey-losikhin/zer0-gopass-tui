package app

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"zer0-gopass-tui/internal/gopass"
)

type fakeLister struct {
	entries []gopass.Entry
	err     error
}

func (f fakeLister) List(context.Context) ([]gopass.Entry, error) { return f.entries, f.err }

type fakeReader struct {
	sets     map[string]gopass.FieldSet
	err      error
	revealed string
}

func (f *fakeReader) ReadManifest(_ context.Context, entry string) (gopass.FieldSet, error) {
	if f.err != nil {
		return gopass.FieldSet{}, f.err
	}
	return f.sets[entry], nil
}

func (f *fakeReader) ResolveField(context.Context, string, string, string) (string, error) {
	return f.revealed, f.err
}

type fakeWriter struct {
	set           gopass.FieldSet
	createdPath   string
	createdFields []gopass.FieldValue
	updatedID     string
	replaced      []gopass.FieldValue
	added         bool
	deletedField  string
	deletedEntry  string
	err           error
}

func (f *fakeWriter) CreateBundle(_ context.Context, path string, fields []gopass.FieldValue) (gopass.FieldSet, error) {
	f.createdPath, f.createdFields = path, fields
	return f.set, f.err
}
func (f *fakeWriter) MigrateBundle(_ context.Context, path string, fields []gopass.FieldValue) (gopass.FieldSet, error) {
	f.createdPath, f.createdFields = path, fields
	return f.set, f.err
}
func (f *fakeWriter) AddField(_ context.Context, _, _ string, _ gopass.FieldValue) (gopass.FieldSet, error) {
	f.added = true
	return f.set, f.err
}
func (f *fakeWriter) UpdateField(_ context.Context, _, _, id string, _ gopass.FieldValue) (gopass.FieldSet, error) {
	f.updatedID = id
	return f.set, f.err
}
func (f *fakeWriter) ReplaceBundle(_ context.Context, _, _ string, fields []gopass.FieldValue) (gopass.FieldSet, error) {
	f.replaced = append([]gopass.FieldValue(nil), fields...)
	return f.set, f.err
}
func (f *fakeWriter) DeleteField(_ context.Context, _, _, id string) (gopass.FieldSet, error) {
	f.deletedField = id
	return f.set, f.err
}
func (f *fakeWriter) DeleteEntry(_ context.Context, entry, _ string) error {
	f.deletedEntry = entry
	return f.err
}
func (f *fakeWriter) DeleteLegacy(_ context.Context, entry string) error {
	f.deletedEntry = entry
	return f.err
}

func testSet() gopass.FieldSet {
	return gopass.FieldSet{Revision: "wire", Fields: []gopass.FieldItem{
		{ID: "user-id", Kind: "username", Name: "Username", Visibility: gopass.VisibilityPublic, Value: "alice"},
		{ID: "pass-id", Kind: "password", Name: "Password", Visibility: gopass.VisibilitySecret},
	}}
}

func loadedModel(t *testing.T, entries []gopass.Entry) (Model, *fakeReader, *fakeWriter) {
	t.Helper()
	r := &fakeReader{sets: map[string]gopass.FieldSet{"work/account": testSet()}, revealed: "secret"}
	w := &fakeWriter{set: testSet()}
	m := NewModel(context.Background(), fakeLister{entries: entries}, r, w)
	updated, _ := m.Update(entriesLoadedMsg{entries: entries})
	m = updated.(Model)
	m.mode = modeList
	m.card = cardModel{}
	return m, r, w
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestFirstEntryAutomaticallyOpensFullEditor(t *testing.T) {
	r := &fakeReader{sets: map[string]gopass.FieldSet{"work/account": testSet()}, revealed: "secret"}
	w := &fakeWriter{set: testSet()}
	m := NewModel(context.Background(), fakeLister{}, r, w)

	updated, cmd := m.Update(entriesLoadedMsg{entries: []gopass.Entry{{Path: "work/account"}}})
	m = updated.(Model)
	if m.mode != modeList || cmd == nil {
		t.Fatalf("mode=%v cmd=%v", m.mode, cmd)
	}
	updated, cmd = m.Update(cmd())
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("field values were not requested")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.card.mode != cardEditAll || m.card.editor.locked != "work/account" {
		t.Fatalf("card=%#v", m.card)
	}
	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.mode != modeCard || cmd != nil {
		t.Fatalf("right panel was not focused: mode=%v cmd=%v", m.mode, cmd)
	}
}

func TestListMovementLoadsSelectedPreview(t *testing.T) {
	r := &fakeReader{sets: map[string]gopass.FieldSet{
		"first":  testSet(),
		"second": testSet(),
	}, revealed: "secret"}
	w := &fakeWriter{set: testSet()}
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "first"}, {Path: "second"}})
	m.reader = r
	m.writer = w
	m.card = newCard(m.ctx, r, w, "first")
	m.card.loading = false

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(Model)
	if m.mode != modeList || m.cursor != 1 || m.card.entry != "second" || cmd == nil {
		t.Fatalf("mode=%v cursor=%d card=%q cmd=%v", m.mode, m.cursor, m.card.entry, cmd)
	}
}

func TestHorizontalKeysSwitchPanelFocus(t *testing.T) {
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	m.card = newCard(m.ctx, m.reader, m.writer, "work/account")
	m.card.loading = false
	m.card.mode = cardEditAll
	m.card.editor = newCreate(m.ctx, m.writer, "work/account")
	m.card.editor.revision = "wire"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(Model)
	if m.mode != modeCard {
		t.Fatalf("right did not focus card: mode=%v", m.mode)
	}
	updated, _ = m.Update(keyRunes("h"))
	m = updated.(Model)
	if m.mode != modeList {
		t.Fatalf("h did not focus list: mode=%v", m.mode)
	}
	updated, _ = m.Update(keyRunes("l"))
	m = updated.(Model)
	if m.mode != modeCard {
		t.Fatalf("l did not focus card: mode=%v", m.mode)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(Model)
	if m.mode != modeList {
		t.Fatalf("left did not focus list: mode=%v", m.mode)
	}
}

func TestEnterOpensFullCardEditor(t *testing.T) {
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	if m.mode != modeCard || cmd == nil || m.quitting {
		t.Fatalf("mode=%v cmd=%v quitting=%v", m.mode, cmd, m.quitting)
	}
	updated, cmd = m.Update(cmd())
	m = updated.(Model)
	if cmd == nil || !m.card.loading {
		t.Fatal("full editor values were not requested")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.card.loading || m.card.mode != cardEditAll || len(m.card.editor.fields) != 20 {
		t.Fatalf("card = %#v", m.card)
	}
	view := m.View()
	if !strings.Contains(view, "alice") || strings.Contains(view, "secret") || !strings.Contains(view, "РЕДАКТИРОВАНИЕ КАРТОЧКИ") {
		t.Fatalf("full editor leaked/missed value: %q", view)
	}
}

func TestQQuitsFromCard(t *testing.T) {
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	m.mode = modeCard
	m.card = newCard(m.ctx, m.reader, m.writer, "work/account")
	m.card.loading = false
	m.card.set = testSet()

	updated, cmd := m.Update(keyRunes("q"))
	m = updated.(Model)
	if !m.quitting || cmd == nil {
		t.Fatalf("quitting=%v cmd=%v", m.quitting, cmd)
	}
}

func TestQCanBeTypedIntoEditor(t *testing.T) {
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	m.mode = modeCard
	m.card = newCard(m.ctx, m.reader, m.writer, "work/account")
	m.card.loading = false
	m.card.mode = cardEditAll
	m.card.editor = newCreate(m.ctx, m.writer, "work/account")
	m.card.editor.revision = "wire"
	m.card.editor.beginEdit()

	updated, _ := m.Update(keyRunes("q"))
	m = updated.(Model)
	if m.quitting || m.card.editor.input.Value() != "q" {
		t.Fatalf("quitting=%v value=%q", m.quitting, m.card.editor.input.Value())
	}
}

func TestCardRevealIsExplicitAndClearedOnLeave(t *testing.T) {
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	m.mode = modeCard
	m.card = newCard(m.ctx, m.reader, m.writer, "work/account")
	m.card.loading = false
	m.card.set = testSet()
	m.card.cursor = 1

	updated, cmd := m.Update(keyRunes("r"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !strings.Contains(m.View(), "secret") {
		t.Fatal("revealed secret not shown")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeList || m.card.revealed != nil {
		t.Fatalf("secret state not cleared: mode=%v card=%#v", m.mode, m.card)
	}
}

func TestListDeleteRequiresConfirmation(t *testing.T) {
	m, _, w := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	updated, cmd := m.Update(keyRunes("d"))
	m = updated.(Model)
	if m.mode != modeListDelete || cmd == nil {
		t.Fatal("delete did not enter loading confirmation")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.loading || !strings.Contains(m.View(), "y — да") {
		t.Fatal("delete confirmation not shown")
	}
	updated, cmd = m.Update(keyRunes("y"))
	m = updated.(Model)
	if cmd == nil {
		t.Fatal("confirmed delete returned nil command")
	}
	updated, _ = m.Update(cmd())
	_ = updated.(Model)
	if w.deletedEntry != "work/account" {
		t.Fatalf("deleted entry = %q", w.deletedEntry)
	}
}

func TestLegacyCardImmediatelyOpensMigrationWizard(t *testing.T) {
	m, r, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	r.err = gopass.ErrManifestNotFound
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.mode != modeCreate || m.create.locked != "work/account" {
		t.Fatalf("migration mode=%v path=%q", m.mode, m.create.locked)
	}
	if strings.Contains(m.View(), "manifest not found") {
		t.Fatalf("legacy backend error is still shown: %q", m.View())
	}
}

func TestListDeletesLegacyEntryAfterConfirmation(t *testing.T) {
	m, r, w := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	r.err = gopass.ErrManifestNotFound
	updated, cmd := m.Update(keyRunes("d"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if !m.legacy || m.err != nil {
		t.Fatalf("legacy=%v err=%v", m.legacy, m.err)
	}
	updated, cmd = m.Update(keyRunes("y"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	_ = updated.(Model)
	if w.deletedEntry != "work/account" {
		t.Fatalf("deleted legacy entry = %q", w.deletedEntry)
	}
}
