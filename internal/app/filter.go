package app

import (
	"strings"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

// filterEntries возвращает подмножество entries, чьи Path содержат query как
// регистронезависимую подпоследовательность символов. Пустой query возвращает
// все записи без изменений.
func filterEntries(entries []gopass.Entry, query string) []gopass.Entry {
	if query == "" {
		return entries
	}

	filtered := make([]gopass.Entry, 0, len(entries))
	for _, entry := range entries {
		if fuzzyMatch(query, entry.Path) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// fuzzyMatch проверяет, встречаются ли все символы query в path в том же
// порядке (не обязательно подряд), регистронезависимо. Сравнение по рунам,
// чтобы корректно работать с не-ASCII символами.
func fuzzyMatch(query, path string) bool {
	q := []rune(strings.ToLower(query))
	p := []rune(strings.ToLower(path))

	qi := 0
	for _, r := range p {
		if qi == len(q) {
			break
		}
		if q[qi] == r {
			qi++
		}
	}
	return qi == len(q)
}

// clampCursor удерживает cursor в границах [0, length-1]. Для пустого списка
// (length == 0) всегда возвращает 0; вызывающий код обязан отдельно проверять
// len(filtered) == 0 перед доступом к элементу по индексу.
func clampCursor(cursor, length int) int {
	if length == 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= length {
		return length - 1
	}
	return cursor
}
