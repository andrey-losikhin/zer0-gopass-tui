package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"zer0-gopass-tui/internal/gopass"
)

type mutationVerifiedMsg struct {
	entries []gopass.Entry
	set     gopass.FieldSet
	notice  error
	err     error
}

func verifyMutationCmd(ctx context.Context, lister gopass.Lister, reader gopass.Reader, entry string, notice error) tea.Cmd {
	return func() tea.Msg {
		set, err := reader.ReadManifest(ctx, entry)
		if err != nil {
			return mutationVerifiedMsg{err: fmt.Errorf("изменение сохранено, но запись не читается: %w", err)}
		}
		entries, err := lister.List(ctx)
		if err != nil {
			return mutationVerifiedMsg{err: fmt.Errorf("изменение сохранено, но список не обновился: %w", err)}
		}
		for _, listed := range entries {
			if listed.Path == entry {
				return mutationVerifiedMsg{entries: entries, set: set, notice: notice}
			}
		}
		return mutationVerifiedMsg{err: fmt.Errorf("gopass не вернул изменённую запись в списке")}
	}
}
