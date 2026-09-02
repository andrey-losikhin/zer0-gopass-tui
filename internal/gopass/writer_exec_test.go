package gopass

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func installFakeGopass(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "argv.log")
	valuePath := filepath.Join(dir, "stdin.value")
	script := `#!/bin/sh
cmd="$1"
last=""
for arg in "$@"; do last="$arg"; done
printf '%s\n' "$*" >> "$GP_LOG"
case "$cmd" in
  insert)
    if [ "$last" = "fail" ]; then echo "backend-secret" >&2; exit 7; fi
    cat > "$GP_VALUE"
    ;;
  rm)
    if [ "$last" = "fail" ]; then echo "backend-secret" >&2; exit 7; fi
    ;;
  show)
    if [ "$last" = "fail" ]; then echo "backend-secret" >&2; exit 7; fi
	printf 'shown-value\n'
    ;;
  list)
    if [ "$last" = "fail" ]; then echo "backend-secret" >&2; exit 7; fi
    if [ "$last" = "existing" ]; then printf 'existing\n'
    elif [ "$last" = "  spaced  " ]; then printf '  spaced  \n'
    else exit 10; fi
    ;;
esac
`
	binary := filepath.Join(dir, "gopass")
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GP_LOG", logPath)
	t.Setenv("GP_VALUE", valuePath)
	return logPath, valuePath
}

func TestExecShowRemovesOnlyGopassTerminalLF(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "gopass")
	content := "#!/bin/sh\nprintf 'line-one\\nline-two\\n\\n'\n"
	if err := os.WriteFile(script, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got, err := execShow(context.Background(), "vault/path")
	if err != nil || string(got) != "line-one\nline-two\n" {
		t.Fatalf("execShow() = %q, %v", got, err)
	}
}

func TestExecWriterBackendUsesFixedArgvAndStdin(t *testing.T) {
	logPath, valuePath := installFakeGopass(t)
	b := execWriterBackend{}
	secret := []byte("dummy-secret-value")
	if err := b.write(context.Background(), "vault/path", secret); err != nil {
		t.Fatalf("write() error = %v", err)
	}
	value, err := os.ReadFile(valuePath)
	if err != nil || string(value) != string(secret) {
		t.Fatalf("stdin value = %q, err=%v", value, err)
	}
	log, _ := os.ReadFile(logPath)
	if !strings.Contains(string(log), "insert --force -- vault/path") || strings.Contains(string(log), string(secret)) {
		t.Fatalf("argv log = %q", log)
	}
	if err := b.remove(context.Background(), "vault/path"); err != nil {
		t.Fatalf("remove() error = %v", err)
	}
}

func TestExecBackendShowExistsAndRedactsStderr(t *testing.T) {
	logPath, _ := installFakeGopass(t)
	b := execWriterBackend{}
	value, err := b.show(context.Background(), "vault/path")
	if err != nil || string(value) != "shown-value" {
		t.Fatalf("show() = %q, %v", value, err)
	}
	log, _ := os.ReadFile(logPath)
	if !strings.Contains(string(log), "show --noparsing --unsafe -- vault/path") {
		t.Fatalf("unsafe show argv missing: %q", log)
	}
	if exists, err := b.exists(context.Background(), "existing"); err != nil || !exists {
		t.Fatalf("exists(existing) = %v, %v", exists, err)
	}
	if exists, err := b.exists(context.Background(), "missing"); err != nil || exists {
		t.Fatalf("exists(missing) = %v, %v", exists, err)
	}
	if exists, err := b.exists(context.Background(), "  spaced  "); err != nil || !exists {
		t.Fatalf("exists(spaced) = %v, %v", exists, err)
	}
	err = b.write(context.Background(), "fail", []byte("safe"))
	if err == nil || strings.Contains(err.Error(), "backend-secret") {
		t.Fatalf("write(fail) error = %v", err)
	}
	for name, call := range map[string]func() error{
		"show":   func() error { _, err := b.show(context.Background(), "fail"); return err },
		"remove": func() error { return b.remove(context.Background(), "fail") },
		"list":   func() error { _, err := b.exists(context.Background(), "fail"); return err },
	} {
		err := call()
		if err == nil || strings.Contains(err.Error(), "backend-secret") {
			t.Fatalf("%s error = %v", name, err)
		}
	}
}
