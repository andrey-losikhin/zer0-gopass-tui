package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"zer0-gopass-tui/internal/gopass"
)

type bitwardenSyncedMsg struct{ err error }

func syncBitwardenCmd(ctx context.Context, syncer bitwardenSyncer, reader gopass.Reader, path string, set gopass.FieldSet) tea.Cmd {
	return func() tea.Msg {
		values := make([]gopass.FieldValue, len(set.Fields))
		for i, field := range set.Fields {
			value := field.Value
			if field.Visibility == gopass.VisibilitySecret {
				var err error
				value, err = reader.ResolveField(ctx, path, set.Revision, field.ID)
				if err != nil {
					return bitwardenSyncedMsg{err: fmt.Errorf("Bitwarden: чтение поля %s: %w", field.Name, err)}
				}
			}
			values[i] = gopass.FieldValue{Kind: field.Kind, Name: field.Name, Visibility: field.Visibility, Multiline: field.Multiline, Value: value}
		}
		return bitwardenSyncedMsg{err: syncer.Upsert(ctx, path, values)}
	}
}
