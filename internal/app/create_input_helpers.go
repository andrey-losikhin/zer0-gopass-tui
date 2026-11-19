package app

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func replaceBundleCmd(ctx context.Context, writer gopass.Writer, path, revision string, fields []gopass.FieldValue) tea.Cmd {
	return func() tea.Msg {
		set, err := writer.ReplaceBundle(ctx, path, revision, fields)
		return createdMsg{path: path, set: set, err: err}
	}
}

// replaceAndMoveCmd сохраняет поля под старым путём и затем переименовывает
// запись: при сбое переноса изменения полей уже не теряются.
func replaceAndMoveCmd(ctx context.Context, writer gopass.Writer, from, to, revision string, fields []gopass.FieldValue, replace bool) tea.Cmd {
	return func() tea.Msg {
		if replace {
			set, err := writer.ReplaceBundle(ctx, from, revision, fields)
			if err != nil {
				return createdMsg{path: from, set: set, err: err}
			}
			revision = set.Revision
		}
		if err := writer.MoveEntry(ctx, from, to, revision); err != nil {
			if replace {
				return createdMsg{path: from, from: from, moveErr: err}
			}
			return createdMsg{path: from, err: fmt.Errorf("переименование: %w", err)}
		}
		return createdMsg{path: to, from: from}
	}
}

func (c *createModel) toggleSecretInput() {
	if c.input.EchoMode == textinput.EchoPassword {
		c.input.EchoMode = textinput.EchoNormal
	} else {
		c.input.EchoMode = textinput.EchoPassword
		c.input.EchoCharacter = '•'
	}
}

func (c createModel) canGenerate() bool {
	return c.cursor > 0 && c.fields[c.cursor-1].Visibility == gopass.VisibilitySecret && !c.fields[c.cursor-1].Multiline
}
