package app

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/totp"
)

type totpTickMsg struct{}

// totpTickCmd перерисовывает карточку раз в секунду, пока раскрыт TOTP secret.
func totpTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return totpTickMsg{} })
}

func (c cardModel) hasRevealedTOTP() bool {
	for _, field := range c.set.Fields {
		if _, ok := c.revealed[field.ID]; ok && field.Kind == "totp_secret" {
			return true
		}
	}
	return false
}

// totpDisplay показывает текущий код вместо самого секрета.
func totpDisplay(secret string, now time.Time) string {
	params, err := totp.Parse(secret)
	if err != nil {
		return "некорректный TOTP secret"
	}
	code, remaining, err := totp.Code(params, now)
	if err != nil {
		return "некорректный TOTP secret"
	}
	return fmt.Sprintf("код %s (ещё %d с)", code, remaining)
}
