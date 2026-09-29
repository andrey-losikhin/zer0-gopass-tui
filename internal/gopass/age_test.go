package gopass

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPasswordChangedAtFallsBackToMtimeWithoutGit(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	script := "#!/bin/sh\n[ \"$1\" = config ] && printf '%s\\n' \"$GP_ROOT\"\n"
	if err := os.WriteFile(filepath.Join(bin, "gopass"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GP_ROOT", root)
	file := filepath.Join(root, "site", "account.gpg")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	want := time.Unix(1700000000, 0)
	if err := os.Chtimes(file, want, want); err != nil {
		t.Fatal(err)
	}
	at, source, err := ExecReader{}.PasswordChangedAt(context.Background(), "site/account")
	if err != nil || source != "mtime" || !at.Equal(want) {
		t.Fatalf("PasswordChangedAt() = %v, %q, %v", at, source, err)
	}
	if _, _, err := (ExecReader{}).PasswordChangedAt(context.Background(), "site/missing"); err == nil {
		t.Fatal("missing entry accepted")
	}
}
