package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

type duplicatesMsg struct {
	groups  [][]string
	checked int
	failed  int
}

// duplicateState — явная проверка дубликатов: подтверждение, ход и результат.
type duplicateState struct {
	confirm bool
	running bool
	result  *duplicatesMsg
}

// findDuplicatesCmd расшифровывает пароль каждой записи, сразу заменяет его
// SHA-256 и сравнивает только хэши; plaintext не покидает функцию.
func findDuplicatesCmd(ctx context.Context, reader gopass.Reader, entries []gopass.Entry) tea.Cmd {
	paths := make([]string, len(entries))
	for i, entry := range entries {
		paths[i] = entry.Path
	}
	return func() tea.Msg {
		byHash := make(map[[sha256.Size]byte][]string)
		result := duplicatesMsg{}
		for _, path := range paths {
			if ctx.Err() != nil {
				break
			}
			sum, ok, err := passwordHash(ctx, reader, path)
			if err != nil {
				result.failed++
				continue
			}
			if ok {
				result.checked++
				byHash[sum] = append(byHash[sum], path)
			}
		}
		for _, group := range byHash {
			if len(group) > 1 {
				slices.Sort(group)
				result.groups = append(result.groups, group)
			}
		}
		slices.SortFunc(result.groups, func(a, b []string) int { return strings.Compare(a[0], b[0]) })
		return result
	}
}

func passwordHash(ctx context.Context, reader gopass.Reader, path string) ([sha256.Size]byte, bool, error) {
	set, err := reader.ReadManifest(ctx, path)
	if errors.Is(err, gopass.ErrManifestNotFound) {
		lr, ok := reader.(legacyReader)
		if !ok {
			return [sha256.Size]byte{}, false, nil
		}
		fields, err := lr.ReadLegacy(ctx, path)
		if err != nil {
			return [sha256.Size]byte{}, false, err
		}
		for _, field := range fields {
			if field.Kind == "password" {
				return sha256.Sum256([]byte(field.Value)), true, nil
			}
		}
		return [sha256.Size]byte{}, false, nil
	}
	if err != nil {
		return [sha256.Size]byte{}, false, err
	}
	for _, field := range set.Fields {
		if field.Kind == "password" {
			value, err := reader.ResolveField(ctx, path, set.Revision, field.ID)
			if err != nil {
				return [sha256.Size]byte{}, false, err
			}
			return sha256.Sum256([]byte(value)), true, nil
		}
	}
	return [sha256.Size]byte{}, false, nil
}

func (d duplicateState) view(total int) string {
	switch {
	case d.confirm:
		return fmt.Sprintf("\n\nПроверка дубликатов расшифрует пароли всех %d записей (возможен запрос pinentry).\nПароли сравниваются только по SHA-256 в памяти и не сохраняются.\nПродолжить? y — да, любая другая клавиша — нет", total)
	case d.running:
		return "\n\nПроверяю дубликаты паролей…"
	case d.result != nil:
		var b strings.Builder
		fmt.Fprintf(&b, "\n\nПроверено паролей: %d, ошибок чтения: %d\n", d.result.checked, d.result.failed)
		if len(d.result.groups) == 0 {
			b.WriteString("Одинаковых паролей не найдено.")
		}
		for i, group := range d.result.groups {
			fmt.Fprintf(&b, "%d. %s\n", i+1, strings.Join(group, ", "))
		}
		return b.String()
	}
	return ""
}
