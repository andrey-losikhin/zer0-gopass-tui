package gopass

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installReaderGopass(t *testing.T, manifest Manifest) (string, string) {
	t.Helper()
	dir := t.TempDir()
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestFile := filepath.Join(dir, "manifest")
	publicFile := filepath.Join(dir, "public")
	secretFile := filepath.Join(dir, "secret")
	syncFile := filepath.Join(dir, "sync")
	logFile := filepath.Join(dir, "show.log")
	for path, value := range map[string][]byte{
		manifestFile: raw,
		publicFile:   []byte("alice"),
		secretFile:   []byte("secret-value"),
		syncFile:     []byte("enabled"),
	} {
		if err := os.WriteFile(path, value, 0600); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
cmd="$1"
last=""
for arg in "$@"; do last="$arg"; done
if [ "$cmd" = "list" ]; then printf '%s\n' "$last"; exit 0; fi
if [ "$cmd" = "show" ]; then
  printf '%s\n' "$last" >> "$READER_LOG"
  if [ "$last" = "$MANIFEST_PATH" ]; then cat "$MANIFEST_FILE"; exit 0; fi
  if [ "$last" = "$PUBLIC_PATH" ]; then cat "$PUBLIC_FILE"; exit 0; fi
  if [ "$last" = "$SECRET_PATH" ]; then cat "$SECRET_FILE"; exit 0; fi
  if [ "$last" = "$SYNC_PATH" ]; then cat "$SYNC_FILE"; exit 0; fi
fi
exit 7
`
	binary := filepath.Join(dir, "gopass")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	manifestID, _ := EncodeCanonicalPath("work/account")
	manifestPath := manifestPathPrefix + manifestID
	publicPath := fieldValuePath(manifest.BundleID, manifest.Revision, manifest.Fields[0].ID)
	secretPath := fieldValuePath(manifest.BundleID, manifest.Revision, manifest.Fields[1].ID)
	syncPath := ""
	if len(manifest.Fields) > 2 {
		syncPath = fieldValuePath(manifest.BundleID, manifest.Revision, manifest.Fields[2].ID)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("READER_LOG", logFile)
	t.Setenv("MANIFEST_PATH", manifestPath)
	t.Setenv("PUBLIC_PATH", publicPath)
	t.Setenv("SECRET_PATH", secretPath)
	t.Setenv("SYNC_PATH", syncPath)
	t.Setenv("MANIFEST_FILE", manifestFile)
	t.Setenv("PUBLIC_FILE", publicFile)
	t.Setenv("SECRET_FILE", secretFile)
	t.Setenv("SYNC_FILE", syncFile)
	return logFile, string(raw)
}

func readerManifest() Manifest {
	return Manifest{
		Format: manifestFormat, BundleID: canonicalTestID(1), Revision: canonicalTestID(2),
		Fields: []Field{
			{ID: canonicalTestID(3), Kind: "username", Name: "Username", Visibility: VisibilityPublic},
			{ID: canonicalTestID(4), Kind: "password", Name: "Password", Visibility: VisibilitySecret},
		},
	}
}

func TestExecReaderMasksSecretAndReadsPublic(t *testing.T) {
	m := readerManifest()
	logPath, raw := installReaderGopass(t, m)
	set, err := (ExecReader{}).ReadManifest(context.Background(), "work/account")
	if err != nil {
		t.Fatalf("ReadManifest() error = %v", err)
	}
	if set.Revision != wireRevisionOf([]byte(raw)) || set.Fields[0].Value != "alice" || set.Fields[1].Value != "" {
		t.Fatalf("set = %#v", set)
	}
	log, _ := os.ReadFile(logPath)
	if strings.Contains(string(log), fieldValuePath(m.BundleID, m.Revision, m.Fields[1].ID)) {
		t.Fatalf("secret value was read implicitly: %q", log)
	}
}

func TestExecReaderHidesBitwardenSyncMarker(t *testing.T) {
	m := readerManifest()
	m.Fields = append(m.Fields, Field{ID: canonicalTestID(5), Kind: "custom", Name: BitwardenSyncFieldName, Visibility: VisibilityPublic})
	logPath, _ := installReaderGopass(t, m)
	set, err := (ExecReader{}).ReadManifest(context.Background(), "work/account")
	if err != nil {
		t.Fatal(err)
	}
	if !set.BitwardenSync || len(set.Fields) != 2 {
		t.Fatalf("sync marker leaked or missed: %#v", set)
	}
	log, _ := os.ReadFile(logPath)
	if !strings.Contains(string(log), fieldValuePath(m.BundleID, m.Revision, m.Fields[2].ID)) {
		t.Fatalf("sync marker was not validated: %q", log)
	}
}

func TestExecReaderResolveChecksRevisionAndMembership(t *testing.T) {
	m := readerManifest()
	_, raw := installReaderGopass(t, m)
	r := ExecReader{}
	value, err := r.ResolveField(context.Background(), "work/account", wireRevisionOf([]byte(raw)), m.Fields[1].ID)
	if err != nil || value != "secret-value" {
		t.Fatalf("ResolveField() = %q, %v", value, err)
	}
	if _, err := r.ResolveField(context.Background(), "work/account", "stale", m.Fields[1].ID); !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("stale error = %v", err)
	}
	if _, err := r.ResolveField(context.Background(), "work/account", wireRevisionOf([]byte(raw)), canonicalTestID(9)); !errors.Is(err, ErrFieldNotFound) {
		t.Fatalf("missing field error = %v", err)
	}
}
