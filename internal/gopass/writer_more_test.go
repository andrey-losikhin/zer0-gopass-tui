package gopass

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestCreateBundleRechecksManifestBeforeReplace(t *testing.T) {
	path, _ := encodedManifestPath("new/account")
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	f.onExists = func(f *fakeWriterBackend) {
		if f.existsCalls == 2 {
			f.data[path] = []byte("concurrent manifest")
		}
	}
	w := ExecWriter{backend: f}

	_, err := w.CreateBundle(context.Background(), "new/account", []FieldValue{{Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "secret"}})
	if !errors.Is(err, errManifestExists) {
		t.Fatalf("CreateBundle() error = %v, want manifest exists", err)
	}
	if got := string(f.data[path]); got != "concurrent manifest" {
		t.Fatalf("manifest = %q, concurrent value was overwritten", got)
	}
}

func TestUpdateFieldReportsCleanupWarningAfterCommit(t *testing.T) {
	f, m, revision := manifestFixture(t)
	failedPath := fieldValuePath(m.BundleID, m.Revision, m.Fields[0].ID)
	f.failRemove[failedPath] = errors.New("remove failed")
	w := ExecWriter{backend: f}

	set, err := w.UpdateField(context.Background(), "work/account", revision, m.Fields[0].ID, FieldValue{Kind: "username", Name: "Username", Visibility: VisibilityPublic, Value: "bob"})
	var cleanup *CleanupError
	if !errors.As(err, &cleanup) || cleanup.Failed != 1 {
		t.Fatalf("UpdateField() error = %v, want CleanupError{Failed:1}", err)
	}
	if set.Revision == "" || set.Fields[0].Value != "bob" {
		t.Fatalf("result = %#v, want committed updated bundle", set)
	}
}

func TestDeleteEntryRechecksRevisionBeforeManifestRemoval(t *testing.T) {
	f, _, revision := manifestFixture(t)
	f.onManifestShow = func(f *fakeWriterBackend) {
		if f.manifestShows == 2 {
			m := Manifest{Format: manifestFormat, BundleID: canonicalTestID(1), Revision: canonicalTestID(5), Fields: []Field{{ID: canonicalTestID(6), Kind: "password", Name: "Password", Visibility: VisibilitySecret}}}
			f.data[f.manifestPath], _ = json.Marshal(m)
		}
	}
	w := ExecWriter{backend: f}

	err := w.DeleteEntry(context.Background(), "work/account", revision)
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("DeleteEntry() error = %v, want ErrStaleRevision", err)
	}
	if len(f.removes) != 0 {
		t.Fatalf("removes = %v, want none", f.removes)
	}
}

func TestParseManifestRejectsDuplicateStandardKind(t *testing.T) {
	m := Manifest{Format: manifestFormat, BundleID: canonicalTestID(1), Revision: canonicalTestID(2), Fields: []Field{
		{ID: canonicalTestID(3), Kind: "password", Name: "Password 1", Visibility: VisibilitySecret},
		{ID: canonicalTestID(4), Kind: "password", Name: "Password 2", Visibility: VisibilitySecret},
	}}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseManifest(raw); err == nil {
		t.Fatal("ParseManifest() error = nil, want duplicate standard kind rejection")
	}
}

func TestValidFieldValueRejectsUnicodeControl(t *testing.T) {
	if err := ValidFieldValue([]byte("a\u0085b"), false); err == nil {
		t.Fatal("ValidFieldValue() error = nil for U+0085 control")
	}
}

func TestCreateBundleWritesCompatibilityEntry(t *testing.T) {
	path, _ := encodedManifestPath("new/account")
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	w := ExecWriter{backend: f}
	set, err := w.CreateBundle(context.Background(), "new/account", []FieldValue{{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "secret",
	}})
	if err != nil {
		t.Fatalf("CreateBundle() error = %v", err)
	}
	if string(f.data["new/account"]) != "zer0-waypass field bundle" {
		t.Fatalf("compatibility entry = %q", f.data["new/account"])
	}
	if len(set.Fields) != 1 || set.Fields[0].Value != "" {
		t.Fatalf("created set = %#v", set)
	}
}

func TestCreateBundleRollsBackWhenCompatibilityWriteFails(t *testing.T) {
	path, _ := encodedManifestPath("new/account")
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{"new/account": errors.New("write failed")}, failRemove: map[string]error{}, manifestPath: path}
	w := ExecWriter{backend: f}
	_, err := w.CreateBundle(context.Background(), "new/account", []FieldValue{{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "secret",
	}})
	if err == nil {
		t.Fatal("CreateBundle() error = nil")
	}
	if _, ok := f.data[path]; ok {
		t.Fatal("manifest remains after compatibility failure")
	}
	if len(f.data) != 0 {
		t.Fatalf("orphan entries remain: %v", f.data)
	}
}

func TestDeleteLegacyRemovesOnlyRequestedEntry(t *testing.T) {
	f := &fakeWriterBackend{data: map[string][]byte{"legacy/account": []byte("secret"), "other": []byte("keep")}, failWrite: map[string]error{}, failRemove: map[string]error{}}
	w := ExecWriter{backend: f}
	if err := w.DeleteLegacy(context.Background(), "legacy/account"); err != nil {
		t.Fatalf("DeleteLegacy() error = %v", err)
	}
	if _, ok := f.data["legacy/account"]; ok {
		t.Fatal("legacy entry was not removed")
	}
	if string(f.data["other"]) != "keep" {
		t.Fatal("unrelated entry changed")
	}
}

func TestCreateBundleRefusesExistingLegacyEntry(t *testing.T) {
	path, _ := encodedManifestPath("legacy/account")
	f := &fakeWriterBackend{data: map[string][]byte{"legacy/account": []byte("original-secret")}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	w := ExecWriter{backend: f}
	_, err := w.CreateBundle(context.Background(), "legacy/account", []FieldValue{{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "new-secret",
	}})
	if !errors.Is(err, errEntryExists) {
		t.Fatalf("CreateBundle() error = %v, want entry exists", err)
	}
	if string(f.data["legacy/account"]) != "original-secret" || len(f.writes) != 0 {
		t.Fatalf("legacy entry changed; data=%q writes=%v", f.data["legacy/account"], f.writes)
	}
}

func TestMigrateBundlePreservesLegacyCompatibilityValue(t *testing.T) {
	path, _ := encodedManifestPath("legacy/account")
	f := &fakeWriterBackend{data: map[string][]byte{"legacy/account": []byte("original-secret")}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	w := ExecWriter{backend: f}
	_, err := w.MigrateBundle(context.Background(), "legacy/account", []FieldValue{{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "new-secret",
	}})
	if err != nil {
		t.Fatalf("MigrateBundle() error = %v", err)
	}
	if string(f.data["legacy/account"]) != "original-secret" {
		t.Fatalf("legacy compatibility value = %q", f.data["legacy/account"])
	}
	if _, ok := f.data[path]; !ok {
		t.Fatal("migrated manifest missing")
	}
}

func TestDeleteLegacyRefusesEntryThatBecameBundle(t *testing.T) {
	path, _ := encodedManifestPath("legacy/account")
	f := &fakeWriterBackend{data: map[string][]byte{"legacy/account": []byte("keep"), path: []byte("manifest")}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	w := ExecWriter{backend: f}
	if err := w.DeleteLegacy(context.Background(), "legacy/account"); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("DeleteLegacy() error = %v, want stale", err)
	}
	if string(f.data["legacy/account"]) != "keep" {
		t.Fatal("compatibility entry was removed")
	}
}
