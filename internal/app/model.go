package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"zer0-gopass-tui/internal/gopass"
)

type mode int

const (
	modeList mode = iota
	modeFilter
	modeCard
	modeCreate
	modeListDelete
)

type entriesLoadedMsg struct{ entries []gopass.Entry }
type errMsg struct{ err error }
type createdVerifiedMsg struct {
	path    string
	entries []gopass.Entry
	set     gopass.FieldSet
	err     error
}

// Model реализует полную keyboard-first оболочку управления field bundles.
type Model struct {
	ctx              context.Context
	lister           gopass.Lister
	reader           gopass.Reader
	writer           gopass.Writer
	bitwarden        bitwardenSyncer
	filter           textinput.Model
	entries          []gopass.Entry
	filtered         []gopass.Entry
	cursor           int
	mode             mode
	card             cardModel
	create           createModel
	delete           gopass.FieldSet
	legacy           bool
	loading          bool
	quitting         bool
	err              error
	notice           error
	width            int
	height           int
	pendingBitwarden bool
}

// NewModel создаёт модель с зависимостями gopass.
func NewModel(ctx context.Context, lister gopass.Lister, reader gopass.Reader, writer gopass.Writer) Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.CharLimit = 256
	return Model{ctx: ctx, lister: lister, reader: reader, writer: writer, bitwarden: newBitwardenClient(), filter: ti, loading: true}
}

// Init запускает асинхронную загрузку списка.
func (m Model) Init() tea.Cmd { return loadEntriesCmd(m.ctx, m.lister) }

func loadEntriesCmd(ctx context.Context, lister gopass.Lister) tea.Cmd {
	return func() tea.Msg {
		entries, err := lister.List(ctx)
		if err != nil {
			return errMsg{err: err}
		}
		return entriesLoadedMsg{entries: entries}
	}
}

// Update обрабатывает сообщения Bubble Tea.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		if key.Type == tea.KeyCtrlC || (commandKey(key) == "q" && !m.acceptingText()) {
			m.quitting = true
			return m, tea.Quit
		}
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		return m, nil
	}
	if m.mode == modeCard {
		return m.updateCard(msg)
	}
	if m.mode == modeList {
		switch msg.(type) {
		case fieldsLoadedMsg, editBundleLoadedMsg:
			card, cmd, _ := m.card.update(msg)
			m.card = card
			return m, cmd
		}
	}
	switch msg := msg.(type) {
	case entriesLoadedMsg:
		m.entries = msg.entries
		m.filtered = filterEntries(m.entries, m.filter.Value())
		m.cursor = clampCursor(m.cursor, len(m.filtered))
		m.loading = false
		m.err = nil
		if len(m.filtered) > 0 {
			entry := m.filtered[m.cursor]
			m.card = newCard(m.ctx, m.reader, m.writer, entry.Path)
			return m, loadFieldsCmd(m.ctx, m.reader, entry.Path)
		}
	case errMsg:
		m.err = msg.err
		m.loading = false
	case createdMsg:
		if msg.err != nil {
			m.create.err = msg.err
			m.create.loading = false
			m.create.status = ""
			return m, nil
		}
		m.create.status = "Проверяю запись в gopass…"
		return m, verifyCreatedCmd(m.ctx, m.lister, m.reader, msg.path)
	case createdVerifiedMsg:
		if msg.err != nil {
			m.create.err = msg.err
			m.create.loading = false
			m.create.status = ""
			return m, nil
		}
		m.card = newCard(m.ctx, m.reader, m.writer, msg.path)
		m.card.loading = false
		m.card.set = msg.set
		m.mode = modeCard
		m.entries = msg.entries
		m.filtered = filterEntries(m.entries, m.filter.Value())
		m.notice = fmt.Errorf("запись сохранена и проверена")
		if m.create.syncBitwarden {
			m.notice = fmt.Errorf("gopass сохранён; синхронизация Bitwarden…")
			m.card.loading = true
			return m, syncBitwardenCmd(m.ctx, m.bitwarden, m.reader, msg.path, msg.set)
		}
	case fieldsLoadedMsg:
		if m.mode == modeListDelete {
			m.loading = false
			m.legacy = errors.Is(msg.err, gopass.ErrManifestNotFound)
			if m.legacy {
				m.err = nil
			} else {
				m.err = msg.err
			}
			m.delete = msg.set
		}
	case mutationMsg:
		if m.mode == modeListDelete {
			m.loading = false
			var cleanup *gopass.CleanupError
			if msg.err != nil && !errors.As(msg.err, &cleanup) {
				m.err = nil
				m.notice = msg.err
				m.mode = modeList
				return m, loadEntriesCmd(m.ctx, m.lister)
			}
			m.err = nil
			m.notice = msg.err
			m.mode = modeList
			return m, loadEntriesCmd(m.ctx, m.lister)
		}
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m Model) acceptingText() bool {
	if m.mode == modeFilter || (m.mode == modeCreate && m.create.editing) {
		return true
	}
	return m.mode == modeCard && m.card.mode == cardEditAll &&
		m.card.editor.editing
}
