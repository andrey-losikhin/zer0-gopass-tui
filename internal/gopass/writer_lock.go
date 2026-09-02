package gopass

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func (w ExecWriter) lock(ctx context.Context, entryPath string) (func(), error) {
	if w.backend != nil && !w.lockEnabled {
		return func() {}, nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("gopass: resolve mutation lock directory: %w", err)
	}
	lockDir := filepath.Join(cacheDir, "zer0-gopass-tui", "locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return nil, fmt.Errorf("gopass: create mutation lock directory: %w", err)
	}
	sum := sha256.Sum256([]byte(entryPath))
	file, err := os.OpenFile(filepath.Join(lockDir, hex.EncodeToString(sum[:])+".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("gopass: open mutation lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			file.Close()
			return nil, fmt.Errorf("gopass: acquire mutation lock: %w", err)
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, fmt.Errorf("gopass: acquire mutation lock: %w", ctx.Err())
		case <-time.After(25 * time.Millisecond):
		}
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
