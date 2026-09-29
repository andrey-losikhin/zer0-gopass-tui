package app

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

// ageReader — необязательная возможность Reader узнать дату смены пароля.
type ageReader interface {
	PasswordChangedAt(context.Context, string) (time.Time, string, error)
}

type ageLoadedMsg struct {
	entry  string
	at     time.Time
	source string
	err    error
}

func loadAgeCmd(ctx context.Context, reader gopass.Reader, entry string) tea.Cmd {
	ar, ok := reader.(ageReader)
	if !ok {
		return nil
	}
	return func() tea.Msg {
		at, source, err := ar.PasswordChangedAt(ctx, entry)
		return ageLoadedMsg{entry: entry, at: at, source: source, err: err}
	}
}

func hasPasswordField(set gopass.FieldSet) bool {
	for _, field := range set.Fields {
		if field.Kind == "password" {
			return true
		}
	}
	return false
}

func passwordAgeText(at time.Time, source string, now time.Time) string {
	days := int(now.Sub(at).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return fmt.Sprintf("Пароль изменён: %s (%d дн. назад, по %s)", at.Format("2006-01-02"), days, source)
}
