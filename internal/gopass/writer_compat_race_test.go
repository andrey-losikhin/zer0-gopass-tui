package gopass

import (
	"context"
	"errors"
	"testing"
)

func TestCreateBundleRechecksCompatibilityBeforeWritingMarker(t *testing.T) {
	path, _ := encodedManifestPath("new/account")
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	f.onExists = func(f *fakeWriterBackend) {
		if f.existsCalls == 4 {
			f.data["new/account"] = []byte("concurrent-secret")
		}
	}
	w := ExecWriter{backend: f}
	_, err := w.CreateBundle(context.Background(), "new/account", []FieldValue{{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "secret",
	}})
	if !errors.Is(err, errEntryExists) {
		t.Fatalf("CreateBundle() error = %v, want entry exists", err)
	}
	if string(f.data["new/account"]) != "concurrent-secret" {
		t.Fatal("concurrent legacy entry was overwritten")
	}
	if _, ok := f.data[path]; ok {
		t.Fatal("manifest was not rolled back")
	}
}

func TestMigrateBundleRollsBackWhenLegacyEntryDisappears(t *testing.T) {
	path, _ := encodedManifestPath("legacy/account")
	f := &fakeWriterBackend{data: map[string][]byte{"legacy/account": []byte("original")}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	f.onExists = func(f *fakeWriterBackend) {
		if f.existsCalls == 4 {
			delete(f.data, "legacy/account")
		}
	}
	w := ExecWriter{backend: f}
	_, err := w.MigrateBundle(context.Background(), "legacy/account", []FieldValue{{
		Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "secret",
	}})
	if err == nil {
		t.Fatal("MigrateBundle() error = nil")
	}
	if _, ok := f.data[path]; ok {
		t.Fatal("manifest remains after legacy entry disappeared")
	}
}
