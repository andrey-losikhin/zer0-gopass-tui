package app

import tea "github.com/charmbracelet/bubbletea"

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeCreate {
		create, cmd, cancel := m.create.update(msg)
		m.create = create
		if cancel {
			m.mode = modeList
		}
		return m, cmd
	}
	if m.mode == modeListDelete {
		if m.loading {
			return m, nil
		}
		if commandKey(msg) == "y" && m.err == nil {
			m.loading = true
			entry := m.filtered[m.cursor]
			if m.legacy {
				return m, deleteLegacyCmd(m.ctx, m.writer, entry.Path)
			}
			return m, deleteEntryCmd(m.ctx, m.writer, entry.Path, m.delete.Revision)
		}
		m.mode = modeList
		return m, nil
	}
	if m.mode == modeFilter {
		return m.updateFilterMode(msg)
	}
	return m.updateListMode(msg)
}

func (m Model) updateListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch commandKey(msg) {
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		m.cursor = clampCursor(m.cursor-1, len(m.filtered))
		return m.openSelectedPreview()
	case "down", "j":
		m.cursor = clampCursor(m.cursor+1, len(m.filtered))
		return m.openSelectedPreview()
	case "/":
		m.mode = modeFilter
		return m, m.filter.Focus()
	case "n":
		m.create = newCreate(m.ctx, m.writer, "")
		m.mode = modeCreate
	case "enter", "tab", "right", "l":
		if len(m.filtered) == 0 {
			return m, nil
		}
		entry := m.filtered[m.cursor]
		if m.card.entry == entry.Path && m.card.mode == cardEditAll && !m.card.loading {
			m.mode = modeCard
			return m, nil
		}
		m.card = newCard(m.ctx, m.reader, m.writer, entry.Path)
		m.mode = modeCard
		return m, loadFieldsCmd(m.ctx, m.reader, entry.Path)
	case "d":
		if len(m.filtered) == 0 {
			return m, nil
		}
		entry := m.filtered[m.cursor]
		m.mode = modeListDelete
		m.loading = true
		m.err = nil
		return m, loadFieldsCmd(m.ctx, m.reader, entry.Path)
	}
	return m, nil
}

func (m Model) openSelectedPreview() (tea.Model, tea.Cmd) {
	if len(m.filtered) == 0 {
		m.card = cardModel{}
		return m, nil
	}
	entry := m.filtered[m.cursor]
	if m.card.entry == entry.Path {
		return m, nil
	}
	m.card = newCard(m.ctx, m.reader, m.writer, entry.Path)
	return m, loadFieldsCmd(m.ctx, m.reader, entry.Path)
}

func (m Model) updateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc || msg.Type == tea.KeyEnter {
		m.filter.Blur()
		m.mode = modeList
		return m, nil
	}
	var cmd tea.Cmd
	m.filter, cmd = m.filter.Update(msg)
	m.filtered = filterEntries(m.entries, m.filter.Value())
	m.cursor = clampCursor(0, len(m.filtered))
	return m, cmd
}
