package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func verifyCreatedCmd(ctx context.Context, lister gopass.Lister, reader gopass.Reader, path string) tea.Cmd {
	return func() tea.Msg {
		set, err := reader.ReadManifest(ctx, path)
		if err != nil {
			return createdVerifiedMsg{path: path, err: fmt.Errorf("запись создана, но не читается: %w", err)}
		}
		entries, err := lister.List(ctx)
		if err != nil {
			return createdVerifiedMsg{path: path, err: fmt.Errorf("запись создана, но список не обновился: %w", err)}
		}
		for _, entry := range entries {
			if entry.Path == path {
				return createdVerifiedMsg{path: path, entries: entries, set: set}
			}
		}
		return createdVerifiedMsg{path: path, err: fmt.Errorf("gopass не вернул сохранённую запись в списке")}
	}
}
