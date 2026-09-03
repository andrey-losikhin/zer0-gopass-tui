package app

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

type fieldForm struct {
	field gopass.FieldValue
	input textinput.Model
	name  bool
}

func newValueForm(field gopass.FieldValue, editName bool) fieldForm {
	input := textinput.New()
	input.CharLimit = 1024 * 1024
	input.Focus()
	if editName {
		input.Prompt = "Имя: "
		input.SetValue(field.Name)
		input.CursorEnd()
	} else {
		configureValueInput(&input, field)
	}
	return fieldForm{field: field, input: input, name: editName}
}

func configureValueInput(input *textinput.Model, field gopass.FieldValue) {
	input.Prompt = "Значение: "
	input.SetValue(field.Value)
	input.CursorEnd()
	if field.Visibility == gopass.VisibilitySecret {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
	} else {
		input.EchoMode = textinput.EchoNormal
	}
}

func (f fieldForm) update(msg tea.KeyMsg) (fieldForm, bool, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		return f, true, nil
	}
	if f.field.Multiline && !f.name && msg.Type == tea.KeyCtrlJ {
		f.input.SetValue(f.input.Value() + "\n")
		f.input.CursorEnd()
		return f, false, nil
	}
	if !f.name && f.field.Visibility == gopass.VisibilitySecret && msg.Type == tea.KeyCtrlR {
		if f.input.EchoMode == textinput.EchoPassword {
			f.input.EchoMode = textinput.EchoNormal
		} else {
			f.input.EchoMode = textinput.EchoPassword
			f.input.EchoCharacter = '•'
		}
		return f, false, nil
	}
	if msg.Type == tea.KeyEnter {
		if f.name {
			f.field.Name = f.input.Value()
			f.name = false
			configureValueInput(&f.input, f.field)
			return f, false, nil
		}
		f.field.Value = f.input.Value()
		return f, true, nil
	}
	var cmd tea.Cmd
	f.input, cmd = f.input.Update(msg)
	return f, false, cmd
}

func (f fieldForm) view() string {
	hint := "Enter сохранить  Esc отмена"
	if f.name {
		hint = "Enter к значению  Esc отмена"
	} else if f.field.Multiline {
		hint = "Ctrl+J новая строка  Enter сохранить  Esc отмена"
	} else if f.field.Visibility == gopass.VisibilitySecret {
		hint = "Ctrl+R показать/скрыть  Enter сохранить  Esc отмена"
	}
	return fmt.Sprintf("%s\n\n%s\n", f.input.View(), hint)
}
