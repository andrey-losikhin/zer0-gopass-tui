package gopass

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func canonicalTestID(b byte) string {
	return base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{b}, 16))
}

type fakeWriterBackend struct {
	data           map[string][]byte
	writes         []string
	removes        []string
	failWrite      map[string]error
	failRemove     map[string]error
	manifestPath   string
	manifestShows  int
	onManifestShow func(*fakeWriterBackend)
	existsCalls    int
	onExists       func(*fakeWriterBackend)
	existsErr      error
	treeRemoves    []string
	failTree       map[string]error
	moves          []string
	failMove       map[string]error
	rootDir        string
}

func (f *fakeWriterBackend) removeTree(_ context.Context, dir string) error {
	f.treeRemoves = append(f.treeRemoves, dir)
	if err := f.failTree[dir]; err != nil {
		return err
	}
	found := false
	for path := range f.data {
		if strings.HasPrefix(path, dir+"/") {
			delete(f.data, path)
			found = true
		}
	}
	if !found {
		return errors.New("not in store")
	}
	return nil
}

func (f *fakeWriterBackend) move(_ context.Context, from, to string) error {
	f.moves = append(f.moves, from+" -> "+to)
	if err := f.failMove[from]; err != nil {
		return err
	}
	value, ok := f.data[from]
	if !ok {
		return errors.New("missing")
	}
	if _, exists := f.data[to]; exists {
		return errors.New("not overwriting")
	}
	f.data[to] = value
	delete(f.data, from)
	return nil
}

func (f *fakeWriterBackend) root(context.Context) (string, error) {
	if f.rootDir == "" {
		return "", errors.New("no root")
	}
	return f.rootDir, nil
}

func (f *fakeWriterBackend) exists(_ context.Context, path string) (bool, error) {
	f.existsCalls++
	if f.onExists != nil {
		f.onExists(f)
	}
	if f.existsErr != nil {
		return false, f.existsErr
	}
	_, ok := f.data[path]
	return ok, nil
}

func (f *fakeWriterBackend) show(_ context.Context, path string) ([]byte, error) {
	if path == f.manifestPath {
		f.manifestShows++
		if f.onManifestShow != nil {
			f.onManifestShow(f)
		}
	}
	value, ok := f.data[path]
	if !ok {
		return nil, errors.New("missing")
	}
	return append([]byte(nil), value...), nil
}

func (f *fakeWriterBackend) write(_ context.Context, path string, value []byte) error {
	f.writes = append(f.writes, path)
	if err := f.failWrite[path]; err != nil {
		return err
	}
	f.data[path] = append([]byte(nil), value...)
	return nil
}

func (f *fakeWriterBackend) remove(_ context.Context, path string) error {
	f.removes = append(f.removes, path)
	if err := f.failRemove[path]; err != nil {
		return err
	}
	delete(f.data, path)
	return nil
}

func manifestFixture(t *testing.T) (*fakeWriterBackend, Manifest, string) {
	t.Helper()
	path, err := encodedManifestPath("work/account")
	if err != nil {
		t.Fatal(err)
	}
	m := Manifest{Format: manifestFormat, BundleID: canonicalTestID(1), Revision: canonicalTestID(2), Fields: []Field{
		{ID: canonicalTestID(3), Kind: "username", Name: "Username", Visibility: VisibilityPublic},
		{ID: canonicalTestID(4), Kind: "password", Name: "Password", Visibility: VisibilitySecret},
	}}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	f.data[path] = raw
	f.data["work/account"] = compatibilityBody("secret")
	f.data[fieldValuePath(m.BundleID, m.Revision, m.Fields[0].ID)] = []byte("alice")
	f.data[fieldValuePath(m.BundleID, m.Revision, m.Fields[1].ID)] = []byte("secret")
	return f, m, wireRevisionOf(raw)
}

func TestCreateBundleValidatesBeforeWriting(t *testing.T) {
	path, _ := encodedManifestPath("new/account")
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path}
	w := ExecWriter{backend: f}

	_, err := w.CreateBundle(context.Background(), "new/account", []FieldValue{{Kind: "password", Name: "Password", Visibility: VisibilitySecret}})
	if err == nil {
		t.Fatal("CreateBundle() error = nil, want invalid empty value")
	}
	if len(f.writes) != 0 {
		t.Fatalf("writes = %v, want none", f.writes)
	}
}

func TestCreateBundleFailsClosedWhenExistenceCheckFails(t *testing.T) {
	path, _ := encodedManifestPath("new/account")
	f := &fakeWriterBackend{data: map[string][]byte{}, failWrite: map[string]error{}, failRemove: map[string]error{}, manifestPath: path, existsErr: errors.New("backend unavailable")}
	w := ExecWriter{backend: f}

	_, err := w.CreateBundle(context.Background(), "new/account", []FieldValue{{Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "secret"}})
	if err == nil {
		t.Fatal("CreateBundle() error = nil, want backend error")
	}
	if len(f.writes) != 0 {
		t.Fatalf("writes = %v, want none", f.writes)
	}
}

func TestUpdateFieldStaleAfterValueWritesCleansNewEntries(t *testing.T) {
	f, m, revision := manifestFixture(t)
	f.onManifestShow = func(f *fakeWriterBackend) {
		if f.manifestShows == 2 {
			f.data[f.manifestPath] = []byte("{\"changed\":true}")
		}
	}
	w := ExecWriter{backend: f}

	_, err := w.UpdateField(context.Background(), "work/account", revision, m.Fields[1].ID, FieldValue{Kind: "password", Name: "Password", Visibility: VisibilitySecret, Value: "new-secret"})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("UpdateField() error = %v, want ErrStaleRevision", err)
	}
	if len(f.treeRemoves) != 1 || len(f.removes) != 0 {
		t.Fatalf("tree removes = %v, removes = %v, want one new-revision removal", f.treeRemoves, f.removes)
	}
	for path := range f.data {
		if strings.HasPrefix(path, f.treeRemoves[0]+"/") {
			t.Fatalf("orphan value %q was not removed", path)
		}
	}
	for _, path := range oldValuePaths(m) {
		if _, ok := f.data[path]; !ok {
			t.Fatalf("old value %q removed after stale failure", path)
		}
	}
}

func TestUpdateFieldManifestFailureKeepsOldBundle(t *testing.T) {
	f, m, revision := manifestFixture(t)
	oldRaw := append([]byte(nil), f.data[f.manifestPath]...)
	f.failWrite[f.manifestPath] = fmt.Errorf("replace failed")
	w := ExecWriter{backend: f}

	_, err := w.UpdateField(context.Background(), "work/account", revision, m.Fields[0].ID, FieldValue{Kind: "username", Name: "Username", Visibility: VisibilityPublic, Value: "bob"})
	if err == nil {
		t.Fatal("UpdateField() error = nil, want replace failure")
	}
	if string(f.data[f.manifestPath]) != string(oldRaw) {
		t.Fatal("old manifest changed after failed replacement")
	}
	for _, field := range m.Fields {
		if _, ok := f.data[fieldValuePath(m.BundleID, m.Revision, field.ID)]; !ok {
			t.Fatalf("old field %s was removed", field.ID)
		}
	}
}
