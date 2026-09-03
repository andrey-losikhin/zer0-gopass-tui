package app

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

type createModel struct {
	ctx           context.Context
	writer        gopass.Writer
	path          textinput.Model
	locked        string
	fields        []gopass.FieldValue
	cursor        int
	input         textinput.Model
	editing       bool
	loading       bool
	status        string
	err           error
	revision      string
	generator     passwordGenerator
	syncBitwarden bool
}

type createdMsg struct {
	path string
	set  gopass.FieldSet
	err  error
}

func newCreate(ctx context.Context, writer gopass.Writer, lockedPath string) createModel {
	path := textinput.New()
	path.Placeholder = "категория/аккаунт"
	path.CharLimit = 512
	path.SetValue(lockedPath)
	c := createModel{ctx: ctx, writer: writer, path: path, locked: lockedPath, fields: allStandardFields()}
	if lockedPath == "" {
		c.beginEdit()
	} else {
		c.cursor = 1
	}
	return c
}

func (c createModel) update(msg tea.KeyMsg) (createModel, tea.Cmd, bool) {
	if c.loading {
		return c, nil, false
	}
	if c.generator.active {
		generator, value, done := c.generator.update(msg)
		c.generator = generator
		if done {
			c.generator.active = false
			if value != "" {
				c.fields[c.cursor-1].Value = value
			}
		}
		return c, nil, false
	}
	if c.editing {
		if msg.Type == tea.KeyCtrlR && c.cursor > 0 && c.fields[c.cursor-1].Visibility == gopass.VisibilitySecret {
			c.toggleSecretInput()
			return c, nil, false
		}
		if msg.Type == tea.KeyCtrlS {
			c = c.commitInput()
			return c.save()
		}
		return c.updateInput(msg)
	}
	if msg.Type == tea.KeyEsc {
		return c, nil, true
	}
	switch commandKey(msg) {
	case "up", "k":
		c.cursor = clampCursor(c.cursor-1, len(c.fields)+1)
		if c.locked != "" && c.cursor == 0 {
			c.cursor = 1
		}
	case "down", "j":
		c.cursor = clampCursor(c.cursor+1, len(c.fields)+1)
	case "enter":
		c.beginEdit()
		return c, c.input.Focus(), false
	case "g":
		if c.canGenerate() {
			c.generator = newPasswordGenerator()
		}
	case "b":
		c.syncBitwarden = !c.syncBitwarden
	}
	if msg.Type == tea.KeyCtrlS {
		return c.save()
	}
	return c, nil, false
}

func (c *createModel) beginEdit() {
	c.input = textinput.New()
	c.input.CharLimit = 1024 * 1024
	value := c.path.Value()
	if c.cursor > 0 {
		field := c.fields[c.cursor-1]
		value = field.Value
		if field.Visibility == gopass.VisibilitySecret {
			c.input.EchoMode = textinput.EchoPassword
			c.input.EchoCharacter = '•'
		}
	}
	c.input.SetValue(value)
	c.input.CursorEnd()
	c.input.Focus()
	c.editing = true
}

func (c createModel) updateInput(msg tea.KeyMsg) (createModel, tea.Cmd, bool) {
	if msg.Type == tea.KeyEsc {
		c.editing = false
		return c, nil, false
	}
	if c.cursor > 0 && c.fields[c.cursor-1].Multiline && msg.Type == tea.KeyCtrlJ {
		c.input.SetValue(c.input.Value() + "\n")
		c.input.CursorEnd()
		return c, nil, false
	}
	if msg.Type == tea.KeyEnter {
		c = c.commitInput()
		return c, nil, false
	}
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	return c, cmd, false
}

func (c createModel) commitInput() createModel {
	if c.cursor == 0 {
		c.path.SetValue(c.input.Value())
	} else {
		c.fields[c.cursor-1].Value = c.input.Value()
	}
	c.editing = false
	c.err = nil
	return c
}

func (c createModel) save() (createModel, tea.Cmd, bool) {
	if _, err := gopass.EncodeCanonicalPath(c.path.Value()); err != nil {
		c.err = fmt.Errorf("проверьте путь записи: %w", err)
		return c, nil, false
	}
	selected := nonEmptyFields(c.fields)
	if len(selected) == 0 {
		c.err = fmt.Errorf("заполните хотя бы одно поле")
		return c, nil, false
	}
	if c.syncBitwarden {
		selected = append(selected, gopass.FieldValue{Kind: "custom", Name: gopass.BitwardenSyncFieldName, Visibility: gopass.VisibilityPublic, Value: "enabled"})
	}
	c.loading = true
	c.status = "Сохраняю зашифрованные поля…"
	if c.locked != "" {
		if c.revision != "" {
			return c, replaceBundleCmd(c.ctx, c.writer, c.path.Value(), c.revision, selected), false
		}
		return c, migrateBundleCmd(c.ctx, c.writer, c.path.Value(), selected), false
	}
	return c, createBundleCmd(c.ctx, c.writer, c.path.Value(), selected), false
}

func nonEmptyFields(fields []gopass.FieldValue) []gopass.FieldValue {
	selected := make([]gopass.FieldValue, 0, len(fields))
	for _, field := range fields {
		if field.Value != "" {
			selected = append(selected, field)
		}
	}
	return selected
}

func createBundleCmd(ctx context.Context, writer gopass.Writer, path string, fields []gopass.FieldValue) tea.Cmd {
	return func() tea.Msg {
		set, err := writer.CreateBundle(ctx, path, fields)
		return createdMsg{path: path, set: set, err: err}
	}
}

func migrateBundleCmd(ctx context.Context, writer gopass.Writer, path string, fields []gopass.FieldValue) tea.Cmd {
	return func() tea.Msg {
		set, err := writer.MigrateBundle(ctx, path, fields)
		return createdMsg{path: path, set: set, err: err}
	}
}
