package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPasswordGeneratorSafeDefaults(t *testing.T) {
	g := newPasswordGenerator()
	password, err := generatePassword(g)
	if err != nil || len(password) != 24 {
		t.Fatalf("password length=%d err=%v", len(password), err)
	}
	if strings.ContainsAny(password, "iloILO01|`'\"{}[]()/\\:;,.<>") {
		t.Fatalf("safe password contains ambiguous character: %q", password)
	}
	for name, chars := range map[string]string{
		"lower": "abcdefghjkmnpqrstuvwxyz", "upper": "ABCDEFGHJKMNPQRSTUVWXYZ",
		"digits": "23456789", "symbols": "!@#$%^&*_-+=?",
	} {
		if !strings.ContainsAny(password, chars) {
			t.Errorf("password misses %s class: %q", name, password)
		}
	}
}

func TestPasswordGeneratorCanEnableDangerousCharacters(t *testing.T) {
	g := newPasswordGenerator()
	g.lower, g.upper, g.digits, g.symbols = false, false, false, true
	g.ambiguous = true
	classes := passwordClasses(g)
	if len(classes) != 1 || !strings.Contains(classes[0], "|") {
		t.Fatalf("dangerous symbols not enabled: %#v", classes)
	}
}

func TestCreateSecretOffersManualOrGeneratedValue(t *testing.T) {
	c := newCreate(nil, &fakeWriter{}, "entry")
	c.cursor = 1 // Password
	c, _, _ = c.update(keyRunes("g"))
	if !c.generator.active || !strings.Contains(c.view(), "ГЕНЕРАТОР ПАРОЛЯ") {
		t.Fatalf("generator not opened: %#v", c.generator)
	}
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyEnter})
	if c.generator.active || len(c.fields[0].Value) != 24 {
		t.Fatalf("generated value was not selected: active=%v length=%d", c.generator.active, len(c.fields[0].Value))
	}
	c, _, _ = c.update(tea.KeyMsg{Type: tea.KeyEnter})
	if !c.editing {
		t.Fatal("manual input is not available after generation")
	}
}
