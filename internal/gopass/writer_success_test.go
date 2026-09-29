package gopass

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
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

func TestManifestVerificationFailureRestoresPreviousManifest(t *testing.T) {
	f, old, revision := manifestFixture(t)
	oldRaw := append([]byte(nil), f.data[f.manifestPath]...)
	f.onManifestShow = func(f *fakeWriterBackend) {
		if f.manifestShows == 3 {
			f.data[f.manifestPath] = []byte("{\"corrupt\":true}")
		}
	}
	w := ExecWriter{backend: f}

	_, err := w.UpdateField(context.Background(), "work/account", revision, old.Fields[0].ID, FieldValue{
		Kind: "username", Name: "Username", Visibility: VisibilityPublic, Value: "bob",
	})
	if err == nil || errors.Is(err, ErrStaleRevision) || !strings.Contains(err.Error(), "restored") {
		t.Fatalf("UpdateField() error = %v, want restored manifest error", err)
	}
	if !bytes.Equal(f.data[f.manifestPath], oldRaw) {
		t.Fatal("previous manifest was not restored")
	}
	for _, path := range oldValuePaths(old) {
		if _, ok := f.data[path]; !ok {
			t.Fatalf("old value %q was removed after unconfirmed manifest", path)
		}
	}
	if len(f.data) != 2+len(old.Fields) {
		t.Fatalf("data entries = %d, new revision was not cleaned", len(f.data))
	}
}

func TestManifestVerificationKeepsConcurrentValidManifest(t *testing.T) {
	f, old, revision := manifestFixture(t)
	concurrent := Manifest{Format: manifestFormat, BundleID: canonicalTestID(1), Revision: canonicalTestID(9), Fields: []Field{{ID: canonicalTestID(10), Kind: "password", Name: "Password", Visibility: VisibilitySecret}}}
	concurrentRaw, _ := json.Marshal(concurrent)
	f.onManifestShow = func(f *fakeWriterBackend) {
		if f.manifestShows == 3 {
			f.data[f.manifestPath] = concurrentRaw
		}
	}
	w := ExecWriter{backend: f}

	_, err := w.UpdateField(context.Background(), "work/account", revision, old.Fields[0].ID, FieldValue{
		Kind: "username", Name: "Username", Visibility: VisibilityPublic, Value: "bob",
	})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("UpdateField() error = %v, want ErrStaleRevision", err)
	}
	if !bytes.Equal(f.data[f.manifestPath], concurrentRaw) {
		t.Fatal("concurrent manifest was overwritten by rollback")
	}
}

func TestSaveWithoutPasswordChangeDoesNotRewriteMainEntry(t *testing.T) {
	f, old, revision := manifestFixture(t)
	w := ExecWriter{backend: f}
	if _, err := w.UpdateField(context.Background(), "work/account", revision, old.Fields[0].ID, FieldValue{
		Kind: "username", Name: "Username", Visibility: VisibilityPublic, Value: "bob",
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range f.writes {
		if path == "work/account" {
			t.Fatal("main entry rewritten although password did not change")
		}
	}
	if len(f.treeRemoves) != 1 || len(f.removes) != 0 {
		t.Fatalf("old revision cleanup: tree=%v single=%v, want exactly one tree removal", f.treeRemoves, f.removes)
	}
}

func TestPasswordChangeUpdatesMainEntryFirstLine(t *testing.T) {
	f, old, revision := manifestFixture(t)
	f.data["work/account"] = []byte("secret\nuser: alice")
	w := ExecWriter{backend: f}
	if _, err := w.UpdateField(context.Background(), "work/account", revision, old.Fields[1].ID, FieldValue{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "rotated",
	}); err != nil {
		t.Fatal(err)
	}
	if string(f.data["work/account"]) != "rotated\nuser: alice" {
		t.Fatal("main entry first line does not match new password")
	}
}

func TestOldMarkerOnlyMainEntryGetsPassword(t *testing.T) {
	got := updatedCompatibility([]byte(legacyCompatibilityMarker), "pw")
	if string(got) != "pw\n"+compatibilityMarker {
		t.Fatalf("updated body = %q", got)
	}
	if got := updatedCompatibility([]byte("\n"+compatibilityMarker), ""); string(got) != "\n"+compatibilityMarker {
		t.Fatalf("passwordless body = %q", got)
	}
}

func TestDeleteEntryContinuesCleanupAndReturnsWarning(t *testing.T) {
	f, old, revision := manifestFixture(t)
	f.data["work/account"] = []byte("compatibility")
	failedPath := fieldValuePath(old.BundleID, old.Revision, old.Fields[0].ID)
	f.failTree = map[string]error{bundleDir(old.BundleID): errors.New("remove failed")}
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
	if _, ok := f.data[failedPath]; !ok {
		t.Fatal("test setup: failed bundle unexpectedly removed")
	}
}
