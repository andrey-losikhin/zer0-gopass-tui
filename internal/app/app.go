package app

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"zer0-gopass-tui/internal/gopass"
)

// Run запускает TUI: строит Model на основе gopass-зависимостей и передаёт управление
// bubbletea, используя stdin/stdout для ввода-вывода. Контекст операций
// отменяется по выходу из Run, чтобы не оставлять процесс gopass
// отсоединённым, если пользователь вышел до завершения загрузки.
// Возвращает ошибку, если программа bubbletea завершилась аварийно.
func Run(stdin io.Reader, stdout io.Writer, lister gopass.Lister, reader gopass.Reader, writer gopass.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := tea.NewProgram(NewModel(ctx, lister, reader, writer), tea.WithInput(stdin), tea.WithOutput(stdout))
	_, err := p.Run()
	if err != nil {
		return err
	}
	return nil
}
