package app

import (
	"fmt"
	"strings"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func (c cardModel) view() string {
	return c.viewRows(12)
}

func (c cardModel) viewRows(rows int) string {
	if c.loading {
		return "Запись: " + c.entry + "\n\nзагрузка...\n"
	}
	if c.legacy {
		return fmt.Sprintf("Запись: %s\n\nЭто legacy-запись без field bundle.\nОшибка: %v\n\nm создать bundle  Esc назад\n", c.entry, c.err)
	}
	if c.fatal {
		return fmt.Sprintf("Запись: %s\n\nОшибка чтения bundle: %v\n\nEsc назад\n", c.entry, c.err)
	}
	if c.mode == cardEdit {
		prefix := "Запись: " + c.entry + "\n\n"
		if c.err != nil {
			prefix += fmt.Sprintf("Ошибка: %v\n\n", c.err)
		}
		return prefix + c.form.view()
	}
	if c.mode == cardEditAll {
		return c.editor.view()
	}
	if c.mode == cardKinds || c.mode == cardCustom {
		return c.kindsView()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Запись: %s\n\n", c.entry)
	if c.err != nil {
		fmt.Fprintf(&b, "Ошибка: %v\n\n", c.err)
	}
	start, end := visibleFieldRange(c.cursor, len(c.set.Fields), max(3, rows))
	if start > 0 {
		b.WriteString("  ↑ ещё поля\n")
	}
	for i := start; i < end; i++ {
		field := c.set.Fields[i]
		prefix := "  "
		if i == c.cursor {
			prefix = "> "
		}
		value := field.Value
		if field.Visibility == gopass.VisibilitySecret {
			value = "••••••••"
			if revealed, ok := c.revealed[field.ID]; ok {
				value = compactValue(revealed)
			}
		} else {
			value = compactValue(value)
		}
		fmt.Fprintf(&b, "%s%s: %s\n", prefix, field.Name, value)
	}
	if end < len(c.set.Fields) {
		b.WriteString("  ↓ ещё поля\n")
	}
	if c.mode == cardConfirmField {
		b.WriteString("\nУдалить выбранное поле? y/N\n")
	} else if c.mode == cardConfirmEntry {
		b.WriteString("\nУдалить всю запись? y/N\n")
	} else {
		b.WriteString("\nr показать  e изменить  a добавить  d удалить поле  x удалить запись  Esc назад\n")
	}
	return b.String()
}

func compactValue(value string) string {
	value = strings.ReplaceAll(value, "\n", " ↵ ")
	value = strings.ReplaceAll(value, "\t", " ⇥ ")
	runes := []rune(value)
	if len(runes) > 120 {
		return string(runes[:119]) + "…"
	}
	return value
}

func visibleFieldRange(cursor, count, window int) (int, int) {
	if count <= window {
		return 0, count
	}
	start := cursor - window/2
	if start < 0 {
		start = 0
	}
	if start+window > count {
		start = count - window
	}
	return start, start + window
}
