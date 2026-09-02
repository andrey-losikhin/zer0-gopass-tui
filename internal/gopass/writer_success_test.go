package gopass

import (
	"context"
	"errors"
	"testing"
)

func TestAddFieldCommitsNewGenerationAndCleansOld(t *testing.T) {
	f, old, revision := manifestFixture(t)
	w := ExecWriter{backend: f}

	set, err := w.AddField(context.Background(), "work/account", revision, FieldValue{
		Kind: "notes", Name: "Notes", Visibility: VisibilityPublic, Multiline: true, Value: "line 1\nline 2",
	})
	if err != nil {
		t.Fatalf("AddField() error = %v", err)
	}
	raw := f.data[f.manifestPath]
	next, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("stored manifest is invalid: %v", err)
	}
	if len(next.Fields) != 3 || next.Revision == old.Revision {
		t.Fatalf("new manifest = %#v, want 3 fields and new revision", next)
	}
	for i, field := range next.Fields {
		if i < len(old.Fields) && field.ID == old.Fields[i].ID {
			t.Fatalf("field %d retained old ID %q", i, field.ID)
		}
		if _, ok := f.data[fieldValuePath(next.BundleID, next.Revision, field.ID)]; !ok {
			t.Fatalf("new value for field %d is missing", i)
		}
	}
	for _, path := range oldValuePaths(old) {
		if _, ok := f.data[path]; ok {
			t.Fatalf("old value %q was not cleaned", path)
		}
	}
	if set.Fields[1].Value != "" {
		t.Fatalf("secret value returned in FieldSet: %q", set.Fields[1].Value)
	}
	if set.Fields[2].Value != "line 1\nline 2" {
		t.Fatalf("public value = %q", set.Fields[2].Value)
	}
}

func TestManifestVerificationFailureKeepsBothGenerations(t *testing.T) {
	f, old, revision := manifestFixture(t)
	f.onManifestShow = func(f *fakeWriterBackend) {
		if f.manifestShows == 3 {
			f.data[f.manifestPath] = []byte("{\"corrupt\":true}")
		}
	}
	w := ExecWriter{backend: f}

	_, err := w.UpdateField(context.Background(), "work/account", revision, old.Fields[0].ID, FieldValue{
		Kind: "username", Name: "Username", Visibility: VisibilityPublic, Value: "bob",
	})
	if err == nil || errors.Is(err, ErrStaleRevision) {
		t.Fatalf("UpdateField() error = %v, want manifest verification error", err)
	}
	for _, path := range oldValuePaths(old) {
		if _, ok := f.data[path]; !ok {
			t.Fatalf("old value %q was removed after unconfirmed manifest", path)
		}
	}
	if len(f.data) < 1+len(old.Fields)*2 {
		t.Fatalf("data entries = %d, new recovery generation was unexpectedly cleaned", len(f.data))
	}
}

func TestDeleteEntryContinuesCleanupAndReturnsWarning(t *testing.T) {
	f, old, revision := manifestFixture(t)
	f.data["work/account"] = []byte("compatibility")
	failedPath := fieldValuePath(old.BundleID, old.Revision, old.Fields[0].ID)
	f.failRemove[failedPath] = errors.New("remove failed")
	w := ExecWriter{backend: f}

	err := w.DeleteEntry(context.Background(), "work/account", revision)
	var cleanup *CleanupError
	if !errors.As(err, &cleanup) || cleanup.Failed != 1 {
		t.Fatalf("DeleteEntry() error = %v", err)
	}
	if _, ok := f.data[f.manifestPath]; ok {
		t.Fatal("manifest remains after delete")
	}
	if _, ok := f.data["work/account"]; ok {
		t.Fatal("compatibility entry remains after delete")
	}
	if _, ok := f.data[fieldValuePath(old.BundleID, old.Revision, old.Fields[1].ID)]; ok {
		t.Fatal("cleanup stopped after first failed value")
	}
	if _, ok := f.data[failedPath]; !ok {
		t.Fatal("test setup: failed value unexpectedly removed")
	}
}
