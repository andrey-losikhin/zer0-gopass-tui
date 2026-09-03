package app

import (
	"context"
	"errors"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

type cardMode int

const (
	cardView cardMode = iota
	cardEdit
	cardEditAll
	cardKinds
	cardCustom
	cardConfirmField
	cardConfirmEntry
)

type cardEvent int

const (
	cardStay cardEvent = iota
	cardLeave
	cardDeleted
	cardMigrate
)

type fieldsLoadedMsg struct {
	entry string
	set   gopass.FieldSet
	err   error
}

type revealMsg struct {
	fieldID string
	value   string
	err     error
}

type mutationMsg struct {
	set         gopass.FieldSet
	err         error
	entryDelete bool
}

type editBundleLoadedMsg struct {
	fields []gopass.FieldValue
	err    error
}

type cardModel struct {
	ctx      context.Context
	reader   gopass.Reader
	writer   gopass.Writer
	entry    string
	set      gopass.FieldSet
	cursor   int
	mode     cardMode
	kind     int
	form     fieldForm
	editor   createModel
	revealed map[string]string
	loading  bool
	legacy   bool
	fatal    bool
	adding   bool
	err      error
}

func newCard(ctx context.Context, reader gopass.Reader, writer gopass.Writer, entry string) cardModel {
	return cardModel{ctx: ctx, reader: reader, writer: writer, entry: entry, revealed: make(map[string]string), loading: true}
}

func loadFieldsCmd(ctx context.Context, reader gopass.Reader, entry string) tea.Cmd {
	return func() tea.Msg {
		set, err := reader.ReadManifest(ctx, entry)
		return fieldsLoadedMsg{entry: entry, set: set, err: err}
	}
}

func (c cardModel) update(msg tea.Msg) (cardModel, tea.Cmd, cardEvent) {
	switch msg := msg.(type) {
	case fieldsLoadedMsg:
		if msg.entry != c.entry {
			return c, nil, cardStay
		}
		c.loading = false
		c.err = msg.err
		c.legacy = errors.Is(msg.err, gopass.ErrManifestNotFound)
		c.fatal = msg.err != nil && !c.legacy
		if msg.err == nil {
			c.set = msg.set
			c.loading = true
			return c, loadBundleValuesCmd(c.ctx, c.reader, c.entry, c.set), cardStay
		}
		return c, nil, cardStay
	case revealMsg:
		c.loading = false
		c.err = msg.err
		if msg.err == nil {
			c.revealed[msg.fieldID] = msg.value
		}
		return c, nil, cardStay
	case editBundleLoadedMsg:
		c.loading = false
		c.err = msg.err
		if msg.err == nil {
			c.editor = newCreate(c.ctx, c.writer, c.entry)
			c.editor.revision = c.set.Revision
			c.editor.syncBitwarden = c.set.BitwardenSync
			c.editor.fields = fullEditorFields(msg.fields)
			c.editor.cursor = 1
			c.editor.editing = false
			c.mode = cardEditAll
		}
		return c, nil, cardStay
	case mutationMsg:
		c.loading = false
		c.err = msg.err
		var cleanup *gopass.CleanupError
		if msg.entryDelete && (msg.err == nil || errors.As(msg.err, &cleanup)) {
			c.revealed = nil
			return c, nil, cardDeleted
		}
		if msg.err != nil && len(msg.set.Fields) == 0 {
			if c.mode != cardEdit {
				c.mode = cardView
			}
			return c, nil, cardStay
		}
		if len(msg.set.Fields) > 0 {
			c.set = msg.set
			c.cursor = clampCursor(c.cursor, len(c.set.Fields))
			c.revealed = make(map[string]string)
		}
		c.adding = false
		c.mode = cardView
		return c, nil, cardStay
	case tea.KeyMsg:
		return c.updateKey(msg)
	}
	return c, nil, cardStay
}

func (c cardModel) updateKey(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	if c.loading {
		return c, nil, cardStay
	}
	if c.fatal {
		if msg.Type == tea.KeyEsc {
			return c, nil, cardLeave
		}
		return c, nil, cardStay
	}
	if c.legacy {
		if msg.Type == tea.KeyEsc {
			return c, nil, cardLeave
		}
		if commandKey(msg) == "m" {
			return c, nil, cardMigrate
		}
		return c, nil, cardStay
	}
	if c.mode == cardEdit {
		return c.updateForm(msg)
	}
	if c.mode == cardEditAll {
		editor, cmd, done := c.editor.update(msg)
		c.editor = editor
		if done {
			c.mode = cardView
		}
		return c, cmd, cardStay
	}
	if c.mode == cardKinds || c.mode == cardCustom {
		return c.updateKinds(msg)
	}
	if c.mode == cardConfirmField || c.mode == cardConfirmEntry {
		return c.updateConfirm(msg)
	}
	return c.updateView(msg)
}
