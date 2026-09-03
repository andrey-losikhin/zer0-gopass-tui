package app

import (
	"context"

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
