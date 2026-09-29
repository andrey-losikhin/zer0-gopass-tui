package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

// legacyReader — необязательная возможность Reader прочитать legacy-запись
// для предзаполнения формы миграции.
type legacyReader interface {
	ReadLegacy(context.Context, string) ([]gopass.FieldValue, error)
}

type legacyLoadedMsg struct {
	entry  string
	fields []gopass.FieldValue
	err    error
}

// loadLegacyCmd расшифровывает legacy-запись только по явному открытию
// карточки или команде миграции.
func loadLegacyCmd(ctx context.Context, reader gopass.Reader, entry string) tea.Cmd {
	return func() tea.Msg {
		lr, ok := reader.(legacyReader)
		if !ok {
			return legacyLoadedMsg{entry: entry}
		}
		fields, err := lr.ReadLegacy(ctx, entry)
		return legacyLoadedMsg{entry: entry, fields: fields, err: err}
	}
}

func (m Model) openMigration(msg legacyLoadedMsg) Model {
	m.card.loading = false
	m.create = newCreate(m.ctx, m.writer, msg.entry)
	if msg.err == nil && len(msg.fields) > 0 {
		m.create.setFields(msg.fields)
	}
	if msg.err != nil {
		m.create.err = fmt.Errorf("не удалось прочитать legacy-запись, форма пуста: %w", msg.err)
	}
	m.mode = modeCreate
	return m
}
