package app

import (
	"context"
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type bitwardenFoundMsg struct {
	path  string
	match bitwardenMatch
	found bool
	err   error
}

type bitwardenDeletedMsg struct {
	match bitwardenMatch
	err   error
}

// bitwardenDeletePrompt — ожидающее подтверждения удаление элемента Bitwarden.
type bitwardenDeletePrompt struct {
	path  string
	match bitwardenMatch
}

// findBitwardenCmd только ищет элемент; удаление выполняется после явного y.
func findBitwardenCmd(ctx context.Context, syncer bitwardenSyncer, path string) tea.Cmd {
	return func() tea.Msg {
		match, found, err := syncer.Find(ctx, path)
		return bitwardenFoundMsg{path: path, match: match, found: found, err: err}
	}
}

func deleteBitwardenCmd(ctx context.Context, syncer bitwardenSyncer, match bitwardenMatch) tea.Cmd {
	return func() tea.Msg {
		return bitwardenDeletedMsg{match: match, err: syncer.Delete(ctx, match.ID)}
	}
}

func relinkBitwardenCmd(ctx context.Context, syncer bitwardenSyncer, oldPath, newPath string) tea.Cmd {
	return func() tea.Msg {
		return bitwardenSyncedMsg{err: syncer.Relink(ctx, oldPath, newPath)}
	}
}

// updateBitwardenDelete обрабатывает поиск, подтверждение и результат удаления.
func (m Model) updateBitwardenDelete(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case bitwardenFoundMsg:
		switch {
		case msg.err != nil:
			m.notice = fmt.Errorf("Bitwarden: связанный элемент не проверен: %w", msg.err)
		case !msg.found:
			m.notice = errors.New("в Bitwarden связанного элемента нет")
		default:
			m.bwDelete = &bitwardenDeletePrompt{path: msg.path, match: msg.match}
		}
		return m, nil, true
	case bitwardenDeletedMsg:
		if msg.err != nil {
			m.notice = fmt.Errorf("Bitwarden: элемент %q не удалён: %w", msg.match.Name, msg.err)
		} else {
			m.notice = fmt.Errorf("Bitwarden: элемент %q перемещён в корзину", msg.match.Name)
		}
		return m, nil, true
	case tea.KeyMsg:
		if m.bwDelete == nil {
			return m, nil, false
		}
		prompt := *m.bwDelete
		m.bwDelete = nil
		if commandKey(msg) == "y" {
			m.notice = errors.New("Bitwarden: удаляю элемент…")
			return m, deleteBitwardenCmd(m.ctx, m.bitwarden, prompt.match), true
		}
		m.notice = fmt.Errorf("Bitwarden: элемент %q оставлен", prompt.match.Name)
		return m, nil, true
	}
	return m, nil, false
}

func (p bitwardenDeletePrompt) view() string {
	return fmt.Sprintf("  Bitwarden: удалить элемент %q (id %s), связанный с %s? Он будет перемещён в корзину. y — да, любая клавиша — нет", p.match.Name, p.match.ID, p.path)
}
