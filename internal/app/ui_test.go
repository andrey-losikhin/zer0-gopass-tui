package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestWorkspaceUsesTwoBorderedPanels(t *testing.T) {
	m, _, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	m = updated.(Model)
	view := m.View()
	for _, want := range []string{"ХРАНИЛИЩЕ", "КАРТОЧКА", "work/account", "╭", "╰"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view misses %q:\n%s", want, view)
		}
	}
}

func TestCreateViewIsFullFieldForm(t *testing.T) {
	m, _, _ := loadedModel(t, nil)
	m.mode = modeCreate
	m.create = newCreate(m.ctx, m.writer, "")
	view := m.View()
	for _, want := range []string{"ХРАНИЛИЩЕ", "НОВАЯ ЗАПИСЬ", "Название записи", "Password", "Username", "Ctrl+S"} {
		if !strings.Contains(view, want) {
			t.Fatalf("create view misses %q:\n%s", want, view)
		}
	}
}

func TestCompactValueDoesNotBreakRowsOrFloodPanel(t *testing.T) {
	got := compactValue(strings.Repeat("x", 200) + "\nnext")
	if strings.Contains(got, "\n") || len([]rune(got)) != 120 || !strings.HasSuffix(got, "…") {
		t.Fatalf("unsafe preview = %q", got)
	}
}

func TestTruncateMiddleKeepsHeadAndTail(t *testing.T) {
	path := "dbdeck-unlock/0e18ebff65e75ddf83d4bf6c528f4c4e7151182a956d5c9ca5fec5d7395a25ac"
	got := truncateMiddle(path, 20)
	if ansi.StringWidth(got) != 20 || !strings.HasPrefix(got, "dbdeck-un") || !strings.HasSuffix(got, "a25ac") {
		t.Fatalf("got %q width=%d", got, ansi.StringWidth(got))
	}
	if truncateMiddle("short", 20) != "short" {
		t.Fatal("short path must stay intact")
	}
}
