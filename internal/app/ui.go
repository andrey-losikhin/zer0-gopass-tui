package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent   = lipgloss.AdaptiveColor{Light: "#7287FD", Dark: "#B4BEFE"} // Catppuccin Lavender.
	colorMuted    = lipgloss.AdaptiveColor{Light: "#9CA0B0", Dark: "#6C7086"} // Catppuccin Overlay 0.
	colorPanel    = lipgloss.AdaptiveColor{Light: "#ACB0BE", Dark: "#585B70"} // Catppuccin Surface 2.
	colorOnMark   = lipgloss.AdaptiveColor{Light: "#EFF1F5", Dark: "#11111B"} // Catppuccin Base/Crust.
	panelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colorPanel).Padding(0, 1)
	focusStyle    = panelStyle.BorderForeground(colorAccent)
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(colorOnMark).Background(colorAccent)
	mutedStyle    = lipgloss.NewStyle().Foreground(colorMuted)
)

func (m Model) dimensions() (int, int) {
	width, height := m.width, m.height
	if width == 0 {
		width = 100
	}
	if height == 0 {
		height = 30
	}
	width = max(72, width)
	height = max(12, height)
	return width, height
}

func (m Model) workspaceView(detail string) string {
	width, height := m.dimensions()
	contentHeight := max(12, height-4)
	leftWidth := max(26, width/3)
	rightWidth := max(34, width-leftWidth-7)
	leftPanel, rightPanel := panelStyle, focusStyle
	if m.mode == modeList || m.mode == modeFilter || m.mode == modeListDelete {
		leftPanel, rightPanel = focusStyle, panelStyle
	}
	left := leftPanel.Width(leftWidth).Height(contentHeight).Render(
		titleStyle.Render("ХРАНИЛИЩЕ") + "\n\n" + m.vaultView(contentHeight-4),
	)
	right := rightPanel.Width(rightWidth).Height(contentHeight).Render(detail)
	help := mutedStyle.Render("  ←/h список • →/l форма • ↑/↓ выбрать • n новая • / поиск • d удалить • q выход")
	if m.notice != nil {
		help += "  " + titleStyle.Render("✓ "+m.notice.Error())
	}
	return lipgloss.JoinVertical(lipgloss.Left, lipgloss.JoinHorizontal(lipgloss.Top, left, right), help)
}

func (m Model) createPanelView() string {
	width, height := m.dimensions()
	content := focusStyle.Width(max(32, width-4)).Height(max(10, height-4)).Render(m.create.viewRows(max(3, height-11)))
	return lipgloss.JoinVertical(lipgloss.Left, content, mutedStyle.Render("  Форма записи • пустые значения не сохраняются"))
}

func (m Model) listDetailView() string {
	if m.loading {
		return titleStyle.Render("КАРТОЧКА") + "\n\nЗагрузка хранилища…"
	}
	if len(m.filtered) == 0 {
		return titleStyle.Render("КАРТОЧКА") + "\n\nНет выбранной записи.\n\nНажмите n, чтобы создать первую."
	}
	entry := m.filtered[m.cursor]
	return fmt.Sprintf("%s\n\n%s\n\n%s", titleStyle.Render("КАРТОЧКА"), entry.Path, mutedStyle.Render("Enter — открыть поля\ne — редактирование после открытия"))
}
