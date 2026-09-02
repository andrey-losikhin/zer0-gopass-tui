package gopass

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Entry представляет одну запись в хранилище gopass, идентифицируемую путём.
type Entry struct {
	Path string
}

// Lister получает список записей из хранилища секретов.
type Lister interface {
	List(ctx context.Context) ([]Entry, error)
}

// ExecLister реализует Lister, вызывая бинарь gopass как внешний процесс.
type ExecLister struct{}

// List выполняет `gopass ls --flat` фиксированным argv (без shell) и
// возвращает распарсенный список записей. Ошибки внешнего вызова
// оборачиваются без утечки stderr бэкенда в текст ошибки.
func (ExecLister) List(ctx context.Context) ([]Entry, error) {
	cmd := exec.CommandContext(ctx, "gopass", "ls", "--flat")
	out, err := cmd.Output()
	if err != nil {
		// Намеренно не включаем err.(*exec.ExitError).Stderr и вывод команды:
		// это может быть чувствительный вывод бэкенда хранилища секретов.
		return nil, fmt.Errorf("gopass ls: %w", err)
	}
	return parseListOutput(string(out)), nil
}

// reservedNamespace — зарезервированный namespace FIELD-CONTRACT.md v1,
// в котором хранятся manifest'ы и value-записи field bundle. Не является
// пользовательской записью и исключается из списка.
const reservedNamespace = ".zer0-waypass/"

// parseListOutput разбирает построчный вывод `gopass ls --flat` в список Entry.
// Строки очищаются от CRLF и пробелов по краям, пустые строки пропускаются.
// Записи из зарезервированного namespace reservedNamespace исключаются.
func parseListOutput(output string) []Entry {
	lines := strings.Split(output, "\n")
	entries := make([]Entry, 0, len(lines))
	for _, line := range lines {
		path := strings.TrimSuffix(line, "\r")
		if path == "" {
			continue
		}
		if err := validateCanonicalPath(path); err != nil {
			continue
		}
		if strings.HasPrefix(path, reservedNamespace) {
			continue
		}
		entries = append(entries, Entry{Path: path})
	}
	return entries
}
