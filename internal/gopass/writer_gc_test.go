package gopass

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// gcFixture зеркалирует fake-данные в файловую структуру store, как её видит gc.
func gcFixture(t *testing.T) (*fakeWriterBackend, Manifest) {
	t.Helper()
	f, m, _ := manifestFixture(t)
	f.rootDir = t.TempDir()
	orphanRevision := revisionDir(m.BundleID, canonicalTestID(20)) + "/" + canonicalTestID(21)
	orphanBundle := bundleDir(canonicalTestID(30)) + "/" + canonicalTestID(31) + "/" + canonicalTestID(32)
	orphanValue := revisionDir(m.BundleID, m.Revision) + "/" + canonicalTestID(40)
	for _, path := range []string{orphanRevision, orphanBundle, orphanValue} {
		f.data[path] = []byte("orphan")
	}
	for path := range f.data {
		file := filepath.Join(f.rootDir, filepath.FromSlash(path)+".gpg")
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(f.rootDir, ".zer0-waypass", "v1", canonicalTestID(50), canonicalTestID(51)), 0700); err != nil {
		t.Fatal(err)
	}
	return f, m
}

func TestGarbageCollectFindsOrphansAndHonoursConfirmation(t *testing.T) {
	f, m := gcFixture(t)
	w := ExecWriter{backend: f}
	report, result, err := w.GarbageCollect(context.Background(), func(GCReport) bool { return false })
	if err != nil {
		t.Fatalf("GarbageCollect() error = %v", err)
	}
	if report.Manifests != 1 || len(report.OrphanBundles) != 1 || len(report.OrphanRevisions) != 1 || len(report.OrphanValues) != 1 || len(report.EmptyDirs) != 2 {
		t.Fatalf("report = %#v", report)
	}
	if result.Removed != 0 || len(f.removes)+len(f.treeRemoves) != 0 {
		t.Fatalf("declined gc removed data: %#v", result)
	}
	_, result, err = w.GarbageCollect(context.Background(), func(GCReport) bool { return true })
	if err != nil || result.Failed != 0 {
		t.Fatalf("apply err=%v result=%#v", err, result)
	}
	for _, path := range oldValuePaths(m) {
		if _, ok := f.data[path]; !ok {
			t.Fatalf("referenced value %q was removed", path)
		}
	}
	if !slices.Contains(f.treeRemoves, bundleDir(canonicalTestID(30))) {
		t.Fatalf("orphan bundle not removed: %v", f.treeRemoves)
	}
	if _, err := os.Stat(filepath.Join(f.rootDir, ".zer0-waypass", "v1", canonicalTestID(50))); !os.IsNotExist(err) {
		t.Fatalf("empty directory remains: %v", err)
	}
}

func TestGarbageCollectAbortsOnInvalidManifest(t *testing.T) {
	f, _ := gcFixture(t)
	f.data[f.manifestPath] = []byte("{broken")
	w := ExecWriter{backend: f}
	called := false
	_, _, err := w.GarbageCollect(context.Background(), func(GCReport) bool { called = true; return true })
	if err == nil || called || len(f.treeRemoves)+len(f.removes) != 0 {
		t.Fatalf("err=%v confirm called=%v removes=%v/%v", err, called, f.treeRemoves, f.removes)
	}
}

func TestGarbageCollectReportsDetachedManifestWithoutOrphaningValues(t *testing.T) {
	f, m := gcFixture(t)
	delete(f.data, "work/account")
	w := ExecWriter{backend: f}
	report, _, err := w.GarbageCollect(context.Background(), func(GCReport) bool { return false })
	if err != nil || len(report.DetachedManifests) != 1 {
		t.Fatalf("err=%v report=%#v", err, report)
	}
	for _, dir := range append(report.OrphanRevisions, report.OrphanBundles...) {
		if dir == revisionDir(m.BundleID, m.Revision) || dir == bundleDir(m.BundleID) {
			t.Fatal("values of detached manifest were marked as orphans")
		}
	}
	_ = json.Valid
}
