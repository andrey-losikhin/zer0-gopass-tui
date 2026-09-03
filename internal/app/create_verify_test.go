package app

import (
	"testing"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestCreateSuccessIsReloadedFromBackend(t *testing.T) {
	m, r, _ := loadedModel(t, nil)
	r.sets["new/account"] = testSet()
	m.lister = fakeLister{entries: []gopass.Entry{{Path: "new/account"}}}
	m.mode = modeCreate
	m.create = newCreate(m.ctx, m.writer, "")
	m.create.loading = true

	updated, cmd := m.Update(createdMsg{path: "new/account", set: testSet()})
	m = updated.(Model)
	if cmd == nil || m.create.status == "" {
		t.Fatal("successful write was not followed by backend verification")
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.mode != modeCard || len(m.entries) != 1 || m.entries[0].Path != "new/account" {
		t.Fatalf("mode=%v entries=%v", m.mode, m.entries)
	}
}

func TestCreateReportsWhenSavedEntryIsMissingFromList(t *testing.T) {
	m, r, _ := loadedModel(t, nil)
	r.sets["new/account"] = testSet()
	m.mode = modeCreate
	m.create = newCreate(m.ctx, m.writer, "")
	m.create.loading = true

	updated, cmd := m.Update(createdMsg{path: "new/account", set: testSet()})
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if m.mode != modeCreate || m.create.loading || m.create.err == nil {
		t.Fatalf("mode=%v loading=%v err=%v", m.mode, m.create.loading, m.create.err)
	}
}
