package app

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type passwordGenerator struct {
	active    bool
	cursor    int
	length    int
	lower     bool
	upper     bool
	digits    bool
	symbols   bool
	ambiguous bool
	err       error
}

func newPasswordGenerator() passwordGenerator {
	return passwordGenerator{active: true, length: 24, lower: true, upper: true, digits: true, symbols: true}
}

func (g passwordGenerator) update(msg tea.KeyMsg) (passwordGenerator, string, bool) {
	if msg.Type == tea.KeyEsc {
		return g, "", true
	}
	switch commandKey(msg) {
	case "up", "k":
		g.cursor = clampCursor(g.cursor-1, 6)
	case "down", "j":
		g.cursor = clampCursor(g.cursor+1, 6)
	case "left", "h":
		if g.cursor == 0 && g.length > 8 {
			g.length--
		}
	case "right", "l":
		if g.cursor == 0 && g.length < 256 {
			g.length++
		}
	case " ":
		g.toggle()
	case "enter", "g":
		value, err := generatePassword(g)
		g.err = err
		if err == nil {
			return g, value, true
		}
	}
	return g, "", false
}

func (g *passwordGenerator) toggle() {
	switch g.cursor {
	case 1:
		g.lower = !g.lower
	case 2:
		g.upper = !g.upper
	case 3:
		g.digits = !g.digits
	case 4:
		g.symbols = !g.symbols
	case 5:
		g.ambiguous = !g.ambiguous
	}
}

func (g passwordGenerator) view() string {
	rows := []string{fmt.Sprintf("Длина: %d", g.length), flag("Строчные", g.lower), flag("Прописные", g.upper), flag("Цифры", g.digits), flag("Спецсимволы", g.symbols), flag("Неоднозначные (опасные)", g.ambiguous)}
	var b strings.Builder
	b.WriteString("ГЕНЕРАТОР ПАРОЛЯ\n\n")
	if g.err != nil {
		fmt.Fprintf(&b, "Ошибка: %v\n\n", g.err)
	}
	for i, row := range rows {
		prefix := "  "
		if i == g.cursor {
			prefix = "› "
		}
		b.WriteString(prefix + row + "\n")
	}
	b.WriteString("\n←/→ длина  Space переключить  Enter сгенерировать  Esc отмена")
	return b.String()
}

func flag(name string, enabled bool) string {
	mark := " "
	if enabled {
		mark = "x"
	}
	return fmt.Sprintf("[%s] %s", mark, name)
}

func generatePassword(g passwordGenerator) (string, error) {
	classes := passwordClasses(g)
	if len(classes) == 0 {
		return "", fmt.Errorf("выберите хотя бы один набор символов")
	}
	if g.length < len(classes) {
		return "", fmt.Errorf("длина меньше числа выбранных наборов")
	}
	password := make([]byte, 0, g.length)
	all := ""
	for _, class := range classes {
		char, err := randomChar(class)
		if err != nil {
			return "", err
		}
		password, all = append(password, char), all+class
	}
	for len(password) < g.length {
		char, err := randomChar(all)
		if err != nil {
			return "", err
		}
		password = append(password, char)
	}
	for i := len(password) - 1; i > 0; i-- {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", fmt.Errorf("генератор случайных чисел: %w", err)
		}
		j := int(n.Int64())
		password[i], password[j] = password[j], password[i]
	}
	return string(password), nil
}

func passwordClasses(g passwordGenerator) []string {
	classes := make([]string, 0, 4)
	add := func(enabled bool, safe, dangerous string) {
		if enabled {
			if g.ambiguous {
				safe += dangerous
			}
			classes = append(classes, safe)
		}
	}
	add(g.lower, "abcdefghjkmnpqrstuvwxyz", "ilo")
	add(g.upper, "ABCDEFGHJKMNPQRSTUVWXYZ", "ILO")
	add(g.digits, "23456789", "01")
	add(g.symbols, "!@#$%^&*_-+=?", "|`'\"{}[]()/\\:;,.<>")
	return classes
}

func randomChar(alphabet string) (byte, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
	if err != nil {
		return 0, fmt.Errorf("генератор случайных чисел: %w", err)
	}
	return alphabet[n.Int64()], nil
}
