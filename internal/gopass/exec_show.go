package gopass

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

// execShow выполняет программное чтение без маскирования safecontent.
func execShow(ctx context.Context, path string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gopass", "show", "--noparsing", "--unsafe", "--", path)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("gopass show: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("gopass show: %w", err)
	}
	out, readErr := io.ReadAll(io.LimitReader(stdout, 1024*1024+1))
	if len(out) > 1024*1024 {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("gopass show: output exceeds size limit")
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, fmt.Errorf("gopass show: %w", readErr)
	}
	if waitErr != nil {
		return nil, fmt.Errorf("gopass show: %w", waitErr)
	}
	// gopass хранит текстовые записи с завершающим LF и возвращает его в pipe.
	// Убираем только служебный LF; дополнительные переводы строк значения
	// остаются нетронутыми.
	return bytes.TrimSuffix(out, []byte("\n")), nil
}
