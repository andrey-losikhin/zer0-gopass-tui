package gopass

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// PasswordChangedAt возвращает время последнего изменения main entry без
// расшифровки: main entry TUI переписывается только при смене пароля (и при
// переименовании). Источник — `git log` store, при его отсутствии — mtime файла.
func (ExecReader) PasswordChangedAt(ctx context.Context, entryPath string) (time.Time, string, error) {
	if err := validateCanonicalPath(entryPath); err != nil {
		return time.Time{}, "", err
	}
	root, err := (execWriterBackend{}).root(ctx)
	if err != nil {
		return time.Time{}, "", err
	}
	var rel string
	var info os.FileInfo
	for _, ext := range storeFileExtensions {
		candidate := filepath.FromSlash(entryPath) + ext
		if stat, err := os.Lstat(filepath.Join(root, candidate)); err == nil && stat.Mode().IsRegular() {
			rel, info = candidate, stat
			break
		}
	}
	if info == nil {
		return time.Time{}, "", fmt.Errorf("gopass: entry file not found in root store")
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		out, err := exec.CommandContext(ctx, "git", "-C", root, "log", "-1", "--format=%ct", "--", rel).Output()
		if err == nil {
			if unix, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return time.Unix(unix, 0), "git", nil
			}
		}
	}
	return info.ModTime(), "mtime", nil
}
