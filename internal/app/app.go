package app

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

// Run запускает TUI: строит Model на основе gopass-зависимостей и передаёт управление
// bubbletea, используя stdin/stdout для ввода-вывода. Контекст операций
// отменяется по выходу из Run, чтобы не оставлять процесс gopass
// отсоединённым, если пользователь вышел до завершения загрузки.
// Возвращает ошибку, если программа bubbletea завершилась аварийно.
func Run(stdin io.Reader, stdout io.Writer, lister gopass.Lister, reader gopass.Reader, writer gopass.Writer) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	model := NewModel(ctx, lister, reader, writer)
	p := tea.NewProgram(model, tea.WithInput(stdin), tea.WithOutput(stdout))
	if client, ok := model.bitwarden.(bitwardenClient); ok && client.server != nil {
		client.server.send = p.Send
		defer client.server.close()
	}
	_, err := p.Run()
	if err != nil {
		return err
	}
	return nil
}
