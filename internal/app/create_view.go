package app

import (
	"fmt"
	"strings"

	"zer0-gopass-tui/internal/gopass"
)

func (c createModel) view() string {
	return c.viewRows(14)
}

func (c createModel) viewRows(rows int) string {
	if c.generator.active {
		return c.generator.view()
	}
	var b strings.Builder
	title := "НОВАЯ ЗАПИСЬ"
	if c.locked != "" {
		title = "МИГРАЦИЯ LEGACY-ЗАПИСИ"
		if c.revision != "" {
			title = "РЕДАКТИРОВАНИЕ КАРТОЧКИ"
		}
	}
	fmt.Fprintf(&b, "%s\n\n", title)
	if c.err != nil {
		fmt.Fprintf(&b, "Ошибка: %v\n\n", c.err)
	}
	if c.loading {
		fmt.Fprintf(&b, "%s\n", c.status)
		return b.String()
	}
	mark := " "
	if c.syncBitwarden {
		mark = "x"
	}
	fmt.Fprintf(&b, "[%s] Синхронизировать с Bitwarden (b)\n\n", mark)
	start, end := visibleFieldRange(c.cursor, len(c.fields)+1, max(3, rows))
	for row := start; row < end; row++ {
		name, value, visibility := c.row(row)
		prefix := "  "
		if row == c.cursor {
			prefix = "› "
		}
		if c.editing && row == c.cursor {
			fmt.Fprintf(&b, "%s%-18s %s\n", prefix, name, c.input.View())
			continue
		}
		if visibility == gopass.VisibilitySecret && value != "" {
			value = strings.Repeat("•", min(12, len([]rune(value))))
		} else {
			value = compactValue(value)
		}
		if value == "" {
			value = "—"
		}
		fmt.Fprintf(&b, "%s%-18s %s\n", prefix, name, value)
	}
	if end < len(c.fields)+1 {
		b.WriteString("  ↓ ещё поля\n")
	}
	b.WriteString("\nEnter ввести  Ctrl+R показать/скрыть  g генератор  ↑/↓ поле  Ctrl+S сохранить  Esc назад\n")
	b.WriteString("Пустые поля не будут записаны.")
	return b.String()
}

func (c createModel) row(index int) (string, string, gopass.Visibility) {
	if index == 0 {
		value := c.path.Value()
		if value == "" {
			value = c.path.Placeholder
		}
		return "Название записи", value, gopass.VisibilityPublic
	}
	field := c.fields[index-1]
	return field.Name, field.Value, field.Visibility
}
