package gopass

import (
	"context"
	"errors"
	"testing"
)

func TestMoveEntryMovesManifestAndMainEntry(t *testing.T) {
	f, _, revision := manifestFixture(t)
	w := ExecWriter{backend: f}
	if err := w.MoveEntry(context.Background(), "work/account", "work/renamed", revision); err != nil {
		t.Fatalf("MoveEntry() error = %v", err)
	}
	target, _ := encodedManifestPath("work/renamed")
	if _, ok := f.data[target]; !ok {
		t.Fatal("manifest was not moved")
	}
	if _, ok := f.data["work/renamed"]; !ok {
		t.Fatal("main entry was not moved")
	}
	if _, ok := f.data[f.manifestPath]; ok {
		t.Fatal("old manifest remains")
	}
}

func TestMoveEntryRefusesExistingTarget(t *testing.T) {
	f, _, revision := manifestFixture(t)
	f.data["work/other"] = []byte("keep")
	w := ExecWriter{backend: f}
	if err := w.MoveEntry(context.Background(), "work/account", "work/other", revision); !errors.Is(err, errEntryExists) {
		t.Fatalf("MoveEntry() error = %v, want entry exists", err)
	}
	if len(f.moves) != 0 {
		t.Fatalf("moves = %v, want none", f.moves)
	}
}

func TestMoveEntryRollsBackManifestWhenMainMoveFails(t *testing.T) {
	f, _, revision := manifestFixture(t)
	f.failMove = map[string]error{"work/account": errors.New("mv failed")}
	w := ExecWriter{backend: f}
	if err := w.MoveEntry(context.Background(), "work/account", "work/renamed", revision); err == nil {
		t.Fatal("MoveEntry() error = nil")
	}
	if _, ok := f.data[f.manifestPath]; !ok {
		t.Fatal("manifest was not moved back")
	}
}

func TestMoveEntryRejectsStaleRevision(t *testing.T) {
	f, _, _ := manifestFixture(t)
	w := ExecWriter{backend: f}
	if err := w.MoveEntry(context.Background(), "work/account", "work/renamed", "stale"); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("MoveEntry() error = %v, want stale", err)
	}
}

func TestMoveLegacyEntryMovesOnlyMainEntry(t *testing.T) {
	f := &fakeWriterBackend{data: map[string][]byte{"legacy/a": []byte("pw")}, failWrite: map[string]error{}, failRemove: map[string]error{}}
	w := ExecWriter{backend: f}
	if err := w.MoveEntry(context.Background(), "legacy/a", "legacy/b", ""); err != nil {
		t.Fatal(err)
	}
	if string(f.data["legacy/b"]) != "pw" || len(f.moves) != 1 {
		t.Fatalf("moves = %v", f.moves)
	}
}
