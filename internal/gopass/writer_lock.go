package gopass

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"
)

// gcLockName — общий lock процессов TUI: мутации держат его в shared-режиме,
// gc — в exclusive. Иначе gc мог бы принять только что записанную, но ещё не
// подключённую к manifest revision за сироту и удалить её.
const gcLockName = "gc.lock"

func (w ExecWriter) lock(ctx context.Context, entryPaths ...string) (func(), error) {
	if w.backend != nil && !w.lockEnabled {
		return func() {}, nil
	}
	lockDir, err := mutationLockDir()
	if err != nil {
		return nil, err
	}
	shared, err := acquireFlock(ctx, filepath.Join(lockDir, gcLockName), syscall.LOCK_SH)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entryPaths))
	for _, entryPath := range entryPaths {
		sum := sha256.Sum256([]byte(entryPath))
		names = append(names, hex.EncodeToString(sum[:])+".lock")
	}
	// Фиксированный порядок исключает взаимную блокировку при rename a->b и b->a.
	slices.Sort(names)
	names = slices.Compact(names)
	releases := []func(){shared}
	release := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	for _, name := range names {
		unlock, err := acquireFlock(ctx, filepath.Join(lockDir, name), syscall.LOCK_EX)
		if err != nil {
			release()
			return nil, err
		}
		releases = append(releases, unlock)
	}
	return release, nil
}

func (w ExecWriter) lockExclusiveGC(ctx context.Context) (func(), error) {
	if w.backend != nil && !w.lockEnabled {
		return func() {}, nil
	}
	lockDir, err := mutationLockDir()
	if err != nil {
		return nil, err
	}
	return acquireFlock(ctx, filepath.Join(lockDir, gcLockName), syscall.LOCK_EX)
}

func mutationLockDir() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("gopass: resolve mutation lock directory: %w", err)
	}
	lockDir := filepath.Join(cacheDir, "zer0-gopass-tui", "locks")
	if err := os.MkdirAll(lockDir, 0700); err != nil {
		return "", fmt.Errorf("gopass: create mutation lock directory: %w", err)
	}
	return lockDir, nil
}

func acquireFlock(ctx context.Context, path string, how int) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("gopass: open mutation lock: %w", err)
	}
	for {
		err = syscall.Flock(int(file.Fd()), how|syscall.LOCK_NB)
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
