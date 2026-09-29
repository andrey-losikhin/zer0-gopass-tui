package app

import (
	"context"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func (c cardModel) updateView(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	switch commandKey(msg) {
	case "esc", "backspace":
		c.revealed = nil
		c.editor = createModel{}
		return c, nil, cardLeave
	case "up", "k":
		c.cursor = clampCursor(c.cursor-1, len(c.set.Fields))
	case "down", "j":
		c.cursor = clampCursor(c.cursor+1, len(c.set.Fields))
	case "r":
		field := c.set.Fields[c.cursor]
		if field.Visibility == gopass.VisibilitySecret {
			c.loading = true
			return c, revealCmd(c.ctx, c.reader, c.entry, c.set.Revision, field.ID), cardStay
		}
	case "e":
		c.loading = true
		return c, loadBundleValuesCmd(c.ctx, c.reader, c.entry, c.set), cardStay
	case "a":
		c.mode, c.kind = cardKinds, 0
	case "d":
		c.mode = cardConfirmField
	case "x":
		c.mode = cardConfirmEntry
	case "R":
		c.mode = cardRename
		c.rename = textinput.New()
		c.rename.Prompt = "Новый путь: "
		c.rename.CharLimit = 512
		c.rename.SetValue(c.entry)
		c.rename.CursorEnd()
		return c, c.rename.Focus(), cardStay
	case "c":
		return c, nil, cardClone
	}
	return c, nil, cardStay
}

func loadBundleValuesCmd(ctx context.Context, reader gopass.Reader, entry string, set gopass.FieldSet) tea.Cmd {
	return func() tea.Msg {
		fields := make([]gopass.FieldValue, len(set.Fields))
		for i, field := range set.Fields {
			value := field.Value
			if field.Visibility == gopass.VisibilitySecret {
				var err error
				value, err = reader.ResolveField(ctx, entry, set.Revision, field.ID)
				if err != nil {
					return editBundleLoadedMsg{err: err}
				}
			}
			fields[i] = gopass.FieldValue{Kind: field.Kind, Name: field.Name, Visibility: field.Visibility, Multiline: field.Multiline, Value: value}
		}
		return editBundleLoadedMsg{fields: fields}
	}
}

func revealCmd(ctx context.Context, reader gopass.Reader, entry, revision, fieldID string) tea.Cmd {
	return func() tea.Msg {
		value, err := reader.ResolveField(ctx, entry, revision, fieldID)
		return revealMsg{fieldID: fieldID, value: value, err: err}
	}
}

type movedMsg struct {
	from string
	to   string
	err  error
}

func (c cardModel) updateRename(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	switch msg.Type {
	case tea.KeyEsc:
		c.mode = cardView
		return c, nil, cardStay
	case tea.KeyEnter:
		target := c.rename.Value()
		if target == c.entry {
			c.mode = cardView
			return c, nil, cardStay
		}
		c.loading = true
		return c, moveEntryCmd(c.ctx, c.writer, c.entry, target, c.set.Revision), cardStay
	}
	var cmd tea.Cmd
	c.rename, cmd = c.rename.Update(msg)
	return c, cmd, cardStay
}

func moveEntryCmd(ctx context.Context, writer gopass.Writer, from, to, revision string) tea.Cmd {
	return func() tea.Msg {
		return movedMsg{from: from, to: to, err: writer.MoveEntry(ctx, from, to, revision)}
	}
}

// cloneTemplate копирует структуру записи и public-значения; secret-поля
// остаются пустыми, поэтому клон не требует расшифровки.
func cloneTemplate(set gopass.FieldSet) []gopass.FieldValue {
	fields := make([]gopass.FieldValue, 0, len(set.Fields))
	for _, field := range set.Fields {
		value := ""
		if field.Visibility == gopass.VisibilityPublic {
			value = field.Value
		}
		fields = append(fields, gopass.FieldValue{Kind: field.Kind, Name: field.Name, Visibility: field.Visibility, Multiline: field.Multiline, Value: value})
	}
	return fields
}
