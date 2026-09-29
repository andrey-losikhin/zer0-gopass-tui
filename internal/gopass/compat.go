package gopass

import (
	"bytes"
	"context"
	"errors"
	"strings"
)

// compatibilityMarker идёт второй строкой main entry записи, созданной TUI:
// первая строка остаётся паролем, чтобы работали `gopass show -c` и `-o`.
const compatibilityMarker = "zer0-waypass: field bundle"

// legacyCompatibilityMarker — прежнее однострочное содержимое main entry без пароля.
const legacyCompatibilityMarker = "zer0-waypass field bundle"

// passwordOf возвращает значение стандартного поля password или "".
func passwordOf(values []FieldValue) string {
	for _, value := range values {
		if value.Kind == "password" {
			return value.Value
		}
	}
	return ""
}

func compatibilityBody(password string) []byte {
	return []byte(password + "\n" + compatibilityMarker)
}

// updatedCompatibility заменяет только первую строку main entry, сохраняя
// остальные строки (например, исходное содержимое мигрированной legacy-записи).
func updatedCompatibility(current []byte, password string) []byte {
	first, rest, hasRest := strings.Cut(string(current), "\n")
	if !hasRest && first == legacyCompatibilityMarker {
		return compatibilityBody(password)
	}
	if !hasRest {
		return []byte(password)
	}
	return []byte(password + "\n" + rest)
}

// syncCompatibility приводит первую строку main entry к паролю manifest.
// Запись выполняется только при расхождении, чтобы не плодить коммиты.
func (w ExecWriter) syncCompatibility(ctx context.Context, entryPath string, values []FieldValue) error {
	current, err := w.store().show(ctx, entryPath)
	if err != nil {
		return err
	}
	next := updatedCompatibility(current, passwordOf(values))
	if bytes.Equal(next, current) {
		return nil
	}
	return w.store().write(ctx, entryPath, next)
}

// commitAndSync фиксирует новую revision и затем синхронизирует main entry.
// Сбой синхронизации не откатывает уже подтверждённый manifest и сообщается
// как предупреждение CleanupError{Compatibility: true}.
func (w ExecWriter) commitAndSync(ctx context.Context, entryPath, manifestPath, expectedRevision string, base Manifest, previousRaw []byte, values []FieldValue) (FieldSet, error) {
	set, err := w.commit(ctx, manifestPath, expectedRevision, base, previousRaw, values)
	var cleanup *CleanupError
	if err != nil && !errors.As(err, &cleanup) {
		return set, err
	}
	if syncErr := w.syncCompatibility(ctx, entryPath, values); syncErr != nil {
		if cleanup == nil {
			cleanup = &CleanupError{}
		}
		cleanup.Compatibility = true
		return set, cleanup
	}
	return set, err
}
