package app

// View отрисовывает активный режим.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.mode == modeCard {
		_, height := m.dimensions()
		rows := max(3, height-12)
		return m.workspaceView(m.card.viewRows(rows))
	}
	if m.mode == modeCreate {
		_, height := m.dimensions()
		return m.workspaceView(m.create.viewRows(max(3, height-12)))
	}
	return m.listView()
}
