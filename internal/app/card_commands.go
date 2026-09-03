package app

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func (c cardModel) updateView(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	switch commandKey(msg) {
	case "esc", "backspace":
		c.revealed = nil
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

func fullEditorFields(current []gopass.FieldValue) []gopass.FieldValue {
	fields := allStandardFields()
	byKind := make(map[gopass.FieldKind]int, len(fields))
	for i, field := range fields {
		byKind[field.Kind] = i
	}
	for _, field := range current {
		if i, ok := byKind[field.Kind]; ok && field.Kind != "custom" {
			fields[i].Value = field.Value
		} else {
			fields = append(fields, field)
		}
	}
	return fields
}

func revealCmd(ctx context.Context, reader gopass.Reader, entry, revision, fieldID string) tea.Cmd {
	return func() tea.Msg {
		value, err := reader.ResolveField(ctx, entry, revision, fieldID)
		return revealMsg{fieldID: fieldID, value: value, err: err}
	}
}
