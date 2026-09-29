package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

// fieldAdder — выбор нового поля формы: недостающие стандартные kind и custom.
type fieldAdder struct {
	active bool
	cursor int
	kinds  []gopass.FieldKind
	naming bool
	custom bool
	name   textinput.Model
	field  gopass.FieldValue
	err    error
}

func newFieldAdder(existing []gopass.FieldValue) fieldAdder {
	present := make(map[gopass.FieldKind]bool, len(existing))
	for _, field := range existing {
		present[field.Kind] = true
	}
	kinds := make([]gopass.FieldKind, 0, len(addableKinds))
	for _, kind := range addableKinds {
		if kind == "custom" || !present[kind] {
			kinds = append(kinds, kind)
		}
	}
	return fieldAdder{active: true, kinds: kinds}
}

func (c createModel) updateAdder(msg tea.KeyMsg) (createModel, tea.Cmd, bool) {
	a := &c.adder
	if msg.Type == tea.KeyEsc {
		c.adder = fieldAdder{}
		return c, nil, false
	}
	if a.naming {
		if msg.Type != tea.KeyEnter {
			var cmd tea.Cmd
			a.name, cmd = a.name.Update(msg)
			return c, cmd, false
		}
		name := strings.TrimSpace(a.name.Value())
		if err := c.validCustomName(name); err != nil {
			a.err = err
			return c, nil, false
		}
		a.field.Name, a.naming, a.err = name, false, nil
		return c, nil, false
	}
	if a.custom {
		switch commandKey(msg) {
		case "v":
			if a.field.Visibility == gopass.VisibilityPublic {
				a.field.Visibility = gopass.VisibilitySecret
			} else {
				a.field.Visibility = gopass.VisibilityPublic
			}
		case "m":
			a.field.Multiline = !a.field.Multiline
		case "enter":
			return c.appendField(a.field)
		}
		return c, nil, false
	}
	switch commandKey(msg) {
	case "up", "k":
		a.cursor = clampCursor(a.cursor-1, len(a.kinds))
	case "down", "j":
		a.cursor = clampCursor(a.cursor+1, len(a.kinds))
	case "enter":
		kind := a.kinds[a.cursor]
		if kind == "custom" {
			a.custom, a.naming = true, true
			a.field = gopass.FieldValue{Kind: "custom", Visibility: gopass.VisibilityPublic}
			a.name = textinput.New()
			a.name.Prompt = "Имя: "
			a.name.CharLimit = 256
			return c, a.name.Focus(), false
		}
		field, _ := gopass.StandardField(kind)
		return c.appendField(field)
	}
	return c, nil, false
}

func (c createModel) appendField(field gopass.FieldValue) (createModel, tea.Cmd, bool) {
	c.adder = fieldAdder{}
	c.fields = append(c.fields, field)
	c.cursor = len(c.fields)
	c.beginEdit()
	return c, c.input.Focus(), false
}

func (c createModel) validCustomName(name string) error {
	if name == "" {
		return fmt.Errorf("укажите имя поля")
	}
	if name == gopass.BitwardenSyncFieldName {
		return fmt.Errorf("это имя поля зарезервировано")
	}
	for _, field := range c.fields {
		if field.Name == name {
			return fmt.Errorf("поле с таким именем уже есть")
		}
	}
	return nil
}

func (a fieldAdder) view() string {
	var b strings.Builder
	if a.err != nil {
		fmt.Fprintf(&b, "Ошибка: %v\n\n", a.err)
	}
	if a.naming {
		fmt.Fprintf(&b, "Новое custom-поле\n\n%s\n\nEnter далее  Esc отмена", a.name.View())
		return b.String()
	}
	if a.custom {
		fmt.Fprintf(&b, "Custom-поле %q\n\nvisibility: %s (v)\nmultiline: %t (m)\n\nEnter добавить  Esc отмена", a.field.Name, a.field.Visibility, a.field.Multiline)
		return b.String()
	}
	b.WriteString("ДОБАВИТЬ ПОЛЕ\n\n")
	for i, kind := range a.kinds {
		prefix := "  "
		if i == a.cursor {
			prefix = "› "
		}
		label := string(kind)
		if field, ok := gopass.StandardField(kind); ok {
			label = field.Name
		}
		fmt.Fprintf(&b, "%s%s\n", prefix, label)
	}
	b.WriteString("\n↑/↓ выбрать  Enter добавить  Esc отмена")
	return b.String()
}
