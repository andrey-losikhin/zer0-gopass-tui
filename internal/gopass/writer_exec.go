package gopass

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

type writerBackend interface {
	show(context.Context, string) ([]byte, error)
	exists(context.Context, string) (bool, error)
	write(context.Context, string, []byte) error
	remove(context.Context, string) error
}

type execWriterBackend struct{}

func (execWriterBackend) show(ctx context.Context, path string) ([]byte, error) {
	return execShow(ctx, path)
}

func (execWriterBackend) exists(ctx context.Context, path string) (bool, error) {
	cmd := exec.CommandContext(ctx, "gopass", "list", "--flat", "--", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("gopass list: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("gopass list: %w", err)
	}
	raw, readErr := io.ReadAll(io.LimitReader(stdout, manifestMaxBytes+1))
	if len(raw) > manifestMaxBytes {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return false, fmt.Errorf("gopass list: output exceeds size limit")
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return false, fmt.Errorf("gopass list: %w", readErr)
	}
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 10 {
			return false, nil
		}
		return false, fmt.Errorf("gopass list: %w", waitErr)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSuffix(line, "\r") == path {
			return true, nil
		}
	}
	return false, nil
}

func (execWriterBackend) write(ctx context.Context, path string, value []byte) error {
	cmd := exec.CommandContext(ctx, "gopass", "insert", "--force", "--", path)
	cmd.Stdin = bytes.NewReader(value)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gopass insert: %w", err)
	}
	return nil
}

func (execWriterBackend) remove(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "gopass", "rm", "--force", "--", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gopass rm: %w", err)
	}
	return nil
}
