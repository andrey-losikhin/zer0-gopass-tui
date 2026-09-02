package app

import (
	"fmt"
	"strings"
)

func (m Model) listView() string {
	detail := m.listDetailView()
	if m.card.entry != "" {
		_, height := m.dimensions()
		detail = m.card.viewRows(max(3, height-12))
	}
	if m.err != nil {
		detail += fmt.Sprintf("\n\nОшибка: %v", m.err)
	}
	if m.notice != nil {
		detail += fmt.Sprintf("\n\n%s", mutedStyle.Render("Статус: "+m.notice.Error()))
	}
	if m.mode == modeListDelete && !m.loading && m.err == nil {
		detail += "\n\nУдалить выбранную запись?  y — да, любая другая клавиша — нет"
	}
	return m.workspaceView(detail)
}

func (m Model) vaultView(height int) string {
	var b strings.Builder
	if m.mode == modeFilter {
		fmt.Fprintf(&b, "%s\n\n", m.filter.View())
	} else {
		query := m.filter.Value()
		if query == "" {
			query = "начните ввод через /"
		}
		fmt.Fprintf(&b, "%s %s\n\n", mutedStyle.Render("Поиск:"), query)
	}
	if m.loading {
		b.WriteString("Загрузка…")
		return b.String()
	}
	if len(m.filtered) == 0 {
		b.WriteString("Записей нет")
		return b.String()
	}
	window := max(5, height-4)
	start, end := visibleFieldRange(m.cursor, len(m.filtered), window)
	if start > 0 {
		b.WriteString(mutedStyle.Render("↑ ещё") + "\n")
	}
	for i := start; i < end; i++ {
		line := "  " + m.filtered[i].Path
		if i == m.cursor {
			line = selectedStyle.Render("› " + m.filtered[i].Path)
		}
		b.WriteString(line + "\n")
	}
	if end < len(m.filtered) {
		b.WriteString(mutedStyle.Render("↓ ещё") + "\n")
	}
	return b.String()
}
