package app

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func (m Model) updateCard(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		m.card.mode = cardView
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
	card, cmd, event := m.card.update(msg)
	m.card = card
	if _, loaded := msg.(fieldsLoadedMsg); loaded && card.legacy {
		m.create = newCreate(m.ctx, m.writer, card.entry)
		m.mode = modeCreate
		return m, nil
	}
	switch event {
	case cardLeave:
		m.mode = modeList
		m.card = cardModel{}
	case cardDeleted:
		m.notice = card.err
		m.mode = modeList
		m.card = cardModel{}
		m.loading = true
		return m, loadEntriesCmd(m.ctx, m.lister)
	case cardMigrate:
		m.create = newCreate(m.ctx, m.writer, m.card.entry)
		m.mode = modeCreate
	}
	return m, cmd
}

func (m Model) cardEditingText() bool {
	if m.card.mode == cardEdit {
		return true
	}
	return m.card.mode == cardEditAll && (m.card.editor.editing || m.card.editor.generator.active)
}
