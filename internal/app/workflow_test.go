package app

import (
	"context"
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

type fakeVault struct {
	sets    map[string]gopass.FieldSet
	values  map[string]map[string]string
	created []gopass.FieldValue
	lists   int
	reads   int
}

func newFakeVault() *fakeVault {
	return &fakeVault{sets: make(map[string]gopass.FieldSet), values: make(map[string]map[string]string)}
}

func (v *fakeVault) List(context.Context) ([]gopass.Entry, error) {
	v.lists++
	entries := make([]gopass.Entry, 0, len(v.sets))
	for path := range v.sets {
		entries = append(entries, gopass.Entry{Path: path})
	}
	return entries, nil
}

func (v *fakeVault) ReadManifest(_ context.Context, path string) (gopass.FieldSet, error) {
	v.reads++
	set, ok := v.sets[path]
	if !ok {
		return gopass.FieldSet{}, gopass.ErrManifestNotFound
	}
	return set, nil
}

func (v *fakeVault) ResolveField(_ context.Context, path, _ string, id string) (string, error) {
	return v.values[path][id], nil
}

func (v *fakeVault) CreateBundle(_ context.Context, path string, fields []gopass.FieldValue) (gopass.FieldSet, error) {
	v.created = append([]gopass.FieldValue(nil), fields...)
	items := make([]gopass.FieldItem, len(fields))
	items = items[:0]
	v.values[path] = make(map[string]string)
	for i, field := range fields {
		id := fmt.Sprintf("field-%d", i)
		if field.Name == gopass.BitwardenSyncFieldName {
			v.values[path][id] = field.Value
			continue
		}
		item := gopass.FieldItem{ID: id, Kind: field.Kind, Name: field.Name, Visibility: field.Visibility, Multiline: field.Multiline}
		if field.Visibility == gopass.VisibilityPublic {
			item.Value = field.Value
		}
		items = append(items, item)
		v.values[path][id] = field.Value
	}
	set := gopass.FieldSet{BundleID: "bundle", Revision: "revision", Fields: items}
	for _, field := range fields {
		set.BitwardenSync = set.BitwardenSync || field.Name == gopass.BitwardenSyncFieldName
	}
	v.sets[path] = set
	return set, nil
}

func (v *fakeVault) MigrateBundle(ctx context.Context, path string, fields []gopass.FieldValue) (gopass.FieldSet, error) {
	return v.CreateBundle(ctx, path, fields)
}
func (v *fakeVault) AddField(context.Context, string, string, gopass.FieldValue) (gopass.FieldSet, error) {
	return gopass.FieldSet{}, nil
}
func (v *fakeVault) UpdateField(context.Context, string, string, string, gopass.FieldValue) (gopass.FieldSet, error) {
	return gopass.FieldSet{}, nil
}
func (v *fakeVault) ReplaceBundle(ctx context.Context, path, _ string, fields []gopass.FieldValue) (gopass.FieldSet, error) {
	return v.CreateBundle(ctx, path, fields)
}
func (v *fakeVault) DeleteField(context.Context, string, string, string) (gopass.FieldSet, error) {
	return gopass.FieldSet{}, nil
}
func (v *fakeVault) DeleteEntry(context.Context, string, string) error { return nil }
func (v *fakeVault) DeleteLegacy(context.Context, string) error        { return nil }

func TestCreateSaveReloadAndOpenWorkflow(t *testing.T) {
	vault := newFakeVault()
	m := NewModel(context.Background(), vault, vault, vault)
	updated, _ := m.Update(entriesLoadedMsg{})
	m = updated.(Model)
	updated, _ = m.Update(keyRunes("n"))
	m = updated.(Model)
	m.create.path.SetValue("github/account")
	m.create.fields[0].Value = "secret"
	m.create.fields[1].Value = "alice"
	m.create.fields[4].Value = "first\nsecond"
	m.create.editing = false

	updated, save := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(Model)
	updated, verify := m.Update(save())
	m = updated.(Model)
	updated, _ = m.Update(verify())
	m = updated.(Model)

	if m.mode != modeCard || len(m.entries) != 1 || m.entries[0].Path != "github/account" {
		t.Fatalf("saved entry not visible: mode=%v entries=%v", m.mode, m.entries)
	}
	if len(vault.created) != 3 || vault.created[0].Value != "secret" || vault.created[2].Value != "first\nsecond" {
		t.Fatalf("saved fields = %#v", vault.created)
	}
	if vault.reads == 0 || vault.lists == 0 {
		t.Fatalf("backend was not reloaded: reads=%d lists=%d", vault.reads, vault.lists)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.mode != modeList || len(m.filtered) != 1 {
		t.Fatalf("entry missing after returning to list: mode=%v filtered=%v", m.mode, m.filtered)
	}
	updated, load := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(Model)
	updated, _ = m.Update(load())
	m = updated.(Model)
	if m.mode != modeCard || len(m.card.set.Fields) != 3 || m.card.set.Fields[1].Value != "alice" {
		t.Fatalf("saved entry did not reopen from backend: mode=%v card=%#v", m.mode, m.card)
	}
}
