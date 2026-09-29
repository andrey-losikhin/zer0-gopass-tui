package app

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func (m Model) updateCard(msg tea.Msg) (tea.Model, tea.Cmd) {
	if loaded, ok := msg.(entriesLoadedMsg); ok {
		m.entries = loaded.entries
		m.filtered = filterEntries(m.entries, m.filter.Value())
		m.loading = false
		for i, entry := range m.filtered {
			if entry.Path == m.card.entry {
				m.cursor = i
			}
		}
		m.cursor = clampCursor(m.cursor, len(m.filtered))
		return m, nil
	}
	if legacy, ok := msg.(legacyLoadedMsg); ok {
		if legacy.entry != m.card.entry {
			return m, nil
		}
		return m.openMigration(legacy), nil
	}
	if key, ok := msg.(tea.KeyMsg); ok && (commandKey(key) == "left" || commandKey(key) == "h") && !m.cardEditingText() {
		m.mode = modeList
		return m, nil
	}
	if created, ok := msg.(createdMsg); ok && m.card.mode == cardEditAll {
		if created.err != nil {
			m.card.editor.loading = false
			m.card.editor.err = created.err
			return m, nil
		}
		m.card.loading = true
		m.pendingBitwarden = m.card.editor.syncBitwarden
		m.pendingUnlink = m.card.set.BitwardenSync && !m.card.editor.syncBitwarden
		m.card.mode = cardView
		m.card.editor = createModel{}
		return m, verifyMutationCmd(m.ctx, m.lister, m.reader, m.card.entry, nil)
	}
	if mutation, ok := msg.(mutationMsg); ok && !mutation.entryDelete {
		var cleanup *gopass.CleanupError
		if len(mutation.set.Fields) > 0 && (mutation.err == nil || errors.As(mutation.err, &cleanup)) {
			m.card.loading = true
			m.pendingBitwarden = m.card.set.BitwardenSync
			return m, verifyMutationCmd(m.ctx, m.lister, m.reader, m.card.entry, mutation.err)
		}
	}
	if verified, ok := msg.(mutationVerifiedMsg); ok {
		m.card.loading = false
		m.card.err = verified.err
		m.card.adding = false
		m.card.mode = cardView
		if verified.err == nil {
			m.card.set = verified.set
			m.card.cursor = clampCursor(m.card.cursor, len(verified.set.Fields))
			m.card.revealed = make(map[string]string)
			m.entries = verified.entries
			m.filtered = filterEntries(m.entries, m.filter.Value())
			m.notice = verified.notice
			if m.pendingUnlink {
				m.pendingUnlink = false
				return m, findBitwardenCmd(m.ctx, m.bitwarden, m.card.entry)
			}
			if m.pendingBitwarden {
				m.pendingBitwarden = false
				m.notice = errors.New("gopass сохранён; синхронизация Bitwarden…")
				m.card.loading = true
				return m, syncBitwardenCmd(m.ctx, m.bitwarden, m.reader, m.card.entry, verified.set)
			}
		}
		return m, nil
	}
	if synced, ok := msg.(bitwardenSyncedMsg); ok {
		m.card.loading = false
		if synced.err != nil {
			m.card.err = fmt.Errorf("gopass сохранён, Bitwarden не синхронизирован: %w", synced.err)
			m.notice = nil
		} else {
			m.card.err = nil
			m.notice = errors.New("gopass и Bitwarden синхронизированы")
		}
		return m, nil
	}
	if moved, ok := msg.(movedMsg); ok {
		m.card.loading = false
		if moved.err != nil {
			m.card.err = fmt.Errorf("переименование: %w", moved.err)
			return m, nil
		}
		synced := m.card.set.BitwardenSync
		m.card = newCard(m.ctx, m.reader, m.writer, moved.to)
		m.notice = fmt.Errorf("запись переименована: %s", moved.to)
		cmds := []tea.Cmd{loadEntriesCmd(m.ctx, m.lister), loadFieldsCmd(m.ctx, m.reader, moved.to)}
		if synced {
			cmds = append(cmds, relinkBitwardenCmd(m.ctx, m.bitwarden, moved.from, moved.to))
		}
		return m, tea.Batch(cmds...)
	}
	card, cmd, event := m.card.update(msg)
	m.card = card
	switch event {
	case cardLeave:
		m.mode = modeList
		m.card = cardModel{}
	case cardDeleted:
		m.notice = card.err
		m.mode = modeList
		deleted, synced := m.card.entry, m.card.set.BitwardenSync
		m.card = cardModel{}
		m.loading = true
		if synced {
			return m, tea.Batch(loadEntriesCmd(m.ctx, m.lister), findBitwardenCmd(m.ctx, m.bitwarden, deleted))
		}
		return m, loadEntriesCmd(m.ctx, m.lister)
	case cardClone:
		m.create = newCreate(m.ctx, m.writer, "")
		m.create.setFields(cloneTemplate(m.card.set))
		m.create.path.SetValue(m.card.entry + "-copy")
		m.create.beginEdit()
		m.mode = modeCreate
		return m, m.create.input.Focus()
	case cardMigrate:
		m.card.loading = true
		return m, loadLegacyCmd(m.ctx, m.reader, m.card.entry)
	}
	return m, cmd
}

func (m Model) cardEditingText() bool {
	if m.card.mode == cardEdit || m.card.mode == cardRename {
		return true
	}
	return m.card.mode == cardEditAll && (m.card.editor.editing || m.card.editor.generator.active || m.card.editor.adder.active)
}
