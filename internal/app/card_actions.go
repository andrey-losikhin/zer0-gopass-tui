package app

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func (c cardModel) updateForm(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	form, done, cmd := c.form.update(msg)
	c.form = form
	if !done {
		return c, cmd, cardStay
	}
	if msg.Type == tea.KeyEsc {
		c.mode = cardView
		return c, nil, cardStay
	}
	if c.form.field.Kind == "custom" && c.form.field.Name == gopass.BitwardenSyncFieldName {
		c.err = fmt.Errorf("это имя поля зарезервировано")
		return c, nil, cardStay
	}
	c.loading = true
	fieldID := ""
	if !c.adding {
		fieldID = c.set.Fields[c.cursor].ID
	}
	return c, updateFieldCmd(c.ctx, c.writer, c.entry, c.set.Revision, fieldID, c.form.field), cardStay
}

func (c cardModel) updateKinds(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	if msg.Type == tea.KeyEsc {
		c.mode = cardView
		return c, nil, cardStay
	}
	if c.mode == cardCustom {
		switch commandKey(msg) {
		case "v":
			if c.form.field.Visibility == gopass.VisibilityPublic {
				c.form.field.Visibility = gopass.VisibilitySecret
			} else {
				c.form.field.Visibility = gopass.VisibilityPublic
			}
		case "m":
			c.form.field.Multiline = !c.form.field.Multiline
		case "enter":
			c.mode = cardEdit
			c.form = newValueForm(c.form.field, true)
		}
		return c, nil, cardStay
	}
	switch commandKey(msg) {
	case "up", "k":
		c.kind = clampCursor(c.kind-1, len(addableKinds))
	case "down", "j":
		c.kind = clampCursor(c.kind+1, len(addableKinds))
	case "enter":
		kind := addableKinds[c.kind]
		if kind == "custom" {
			c.form.field = gopass.FieldValue{Kind: kind, Visibility: gopass.VisibilityPublic}
			c.adding = true
			c.mode = cardCustom
			return c, nil, cardStay
		}
		if c.hasKind(kind) {
			c.err = fmt.Errorf("поле kind %s уже существует", kind)
			c.mode = cardView
			return c, nil, cardStay
		}
		field, _ := gopass.StandardField(kind)
		c.form = newValueForm(field, true)
		c.mode = cardEdit
		c.adding = true
	}
	return c, nil, cardStay
}

func (c cardModel) updateConfirm(msg tea.KeyMsg) (cardModel, tea.Cmd, cardEvent) {
	if commandKey(msg) != "y" {
		c.mode = cardView
		return c, nil, cardStay
	}
	c.loading = true
	if c.mode == cardConfirmEntry {
		return c, deleteEntryCmd(c.ctx, c.writer, c.entry, c.set.Revision), cardStay
	}
	field := c.set.Fields[c.cursor]
	return c, deleteFieldCmd(c.ctx, c.writer, c.entry, c.set.Revision, field.ID), cardStay
}

func (c cardModel) hasKind(kind gopass.FieldKind) bool {
	for _, field := range c.set.Fields {
		if field.Kind == kind && kind != "custom" {
			return true
		}
	}
	return false
}

func (c cardModel) kindsView() string {
	if c.mode == cardCustom {
		return fmt.Sprintf("Новое custom-поле\n\nvisibility: %s (v)\nmultiline: %t (m)\n\nEnter продолжить  Esc отмена\n", c.form.field.Visibility, c.form.field.Multiline)
	}
	var b strings.Builder
	b.WriteString("Выберите тип поля\n\n")
	for i, kind := range addableKinds {
		prefix := "  "
		if i == c.kind {
			prefix = "> "
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, kind)
	}
	b.WriteString("\nEnter выбрать  Esc отмена\n")
	return b.String()
}

func updateFieldCmd(ctx context.Context, writer gopass.Writer, entry, revision, fieldID string, value gopass.FieldValue) tea.Cmd {
	return func() tea.Msg {
		var set gopass.FieldSet
		var err error
		if fieldID == "" {
			set, err = writer.AddField(ctx, entry, revision, value)
		} else {
			set, err = writer.UpdateField(ctx, entry, revision, fieldID, value)
		}
		return mutationMsg{set: set, err: err}
	}
}

func deleteFieldCmd(ctx context.Context, writer gopass.Writer, entry, revision, fieldID string) tea.Cmd {
	return func() tea.Msg {
		set, err := writer.DeleteField(ctx, entry, revision, fieldID)
		return mutationMsg{set: set, err: err}
	}
}

func deleteEntryCmd(ctx context.Context, writer gopass.Writer, entry, revision string) tea.Cmd {
	return func() tea.Msg {
		err := writer.DeleteEntry(ctx, entry, revision)
		return mutationMsg{err: err, entryDelete: true}
	}
}

func deleteLegacyCmd(ctx context.Context, writer gopass.Writer, entry string) tea.Cmd {
	return func() tea.Msg {
		err := writer.DeleteLegacy(ctx, entry)
		return mutationMsg{err: err, entryDelete: true}
	}
}
