package app

import (
	"context"
	"errors"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestCreateShowsAllStandardFieldsWithoutTemplates(t *testing.T) {
	c := newCreate(context.Background(), &fakeWriter{}, "")
	if len(c.fields) != 20 {
		t.Fatalf("field count = %d, want 20", len(c.fields))
	}
	for _, field := range c.fields {
		if field.Kind == "custom" || field.Value != "" {
			t.Fatalf("unexpected initial field: %#v", field)
		}
	}
	view := c.view()
	if view == "" || c.path.Placeholder != "категория/аккаунт" {
		t.Fatalf("view=%q placeholder=%q", view, c.path.Placeholder)
	}
}

func TestCreateFormSavesOnlyFilledFields(t *testing.T) {
	w := &fakeWriter{set: testSet()}
	c := newCreate(context.Background(), w, "")
	c.input.SetValue("new/account")
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyEnter})
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyDown})
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyEnter})
	c.input.SetValue("password-value")
	c, cmd, _ := c.update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil || !c.loading {
		t.Fatalf("save cmd=%v loading=%v err=%v", cmd, c.loading, c.err)
	}
	_ = cmd()
	if w.createdPath != "new/account" || len(w.createdFields) != 1 {
		t.Fatalf("path=%q fields=%#v", w.createdPath, w.createdFields)
	}
}

func TestCreateRejectsAllEmptyFields(t *testing.T) {
	c := newCreate(context.Background(), &fakeWriter{}, "")
	c.path.SetValue("new/account")
	c.editing = false
	c, cmd, _ := c.update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd != nil || c.err == nil {
		t.Fatalf("cmd=%v err=%v", cmd, c.err)
	}
}

func TestSecretInputCanBeRevealedAndMaskedWithoutChangingValue(t *testing.T) {
	c := newCreate(context.Background(), &fakeWriter{}, "entry")
	c.cursor = 1
	c.beginEdit()
	c.input.SetValue("visible-secret")
	if c.input.EchoMode != textinput.EchoPassword {
		t.Fatal("secret input is not masked initially")
	}
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if c.input.EchoMode != textinput.EchoNormal || c.input.Value() != "visible-secret" {
		t.Fatalf("reveal lost value: mode=%v value=%q", c.input.EchoMode, c.input.Value())
	}
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if c.input.EchoMode != textinput.EchoPassword || c.input.Value() != "visible-secret" {
		t.Fatalf("mask lost value: mode=%v value=%q", c.input.EchoMode, c.input.Value())
	}
}

func TestCreateRetryAfterBackendErrorKeepsWholeForm(t *testing.T) {
	m, _, w := loadedModel(t, nil)
	w.err = errors.New("backend unavailable")
	m.mode = modeCreate
	m.create = newCreate(m.ctx, w, "")
	m.create.path.SetValue("new/account")
	m.create.fields[0].Value = "secret"
	m.create.loading = true

	updated, _ := m.Update(createdMsg{path: "new/account", err: w.err})
	m = updated.(Model)
	if m.create.loading || m.create.fields[0].Value != "secret" || m.create.err == nil {
		t.Fatalf("retry state = %#v", m.create)
	}
}

func TestCustomFieldMetadataToggles(t *testing.T) {
	c := newCard(context.Background(), &fakeReader{}, &fakeWriter{}, "entry")
	c.loading = false
	c.set = gopass.FieldSet{Revision: "wire", Fields: []gopass.FieldItem{{ID: "id", Kind: "username", Name: "Username", Visibility: gopass.VisibilityPublic}}}
	c.mode = cardKinds
	c.kind = len(addableKinds) - 1
	c, _, _ = c.updateKey(tea.KeyMsg{Type: tea.KeyEnter})
	c, _, _ = c.updateKey(keyRunes("v"))
	c, _, _ = c.updateKey(keyRunes("m"))
	if c.form.field.Visibility != gopass.VisibilitySecret || !c.form.field.Multiline {
		t.Fatalf("custom metadata = %#v", c.form.field)
	}
}

func TestVisibleFieldRangeKeepsCursorInWindow(t *testing.T) {
	start, end := visibleFieldRange(18, 21, 14)
	if start > 18 || end <= 18 || end-start != 14 {
		t.Fatalf("range = [%d,%d), cursor not visible", start, end)
	}
}
