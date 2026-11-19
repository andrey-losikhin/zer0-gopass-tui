package app

import (
	"context"
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBitwardenServerStartAsksUnlockOnlyWhenUnreachable(t *testing.T) {
	var requests int
	s := &bitwardenServer{port: "18087", send: func(msg tea.Msg) {
		requests++
		msg.(bitwardenUnlockRequestMsg).reply <- bitwardenUnlockResult{err: errors.New("cancelled")}
	}}
	if err := s.start(context.Background(), func(context.Context) bool { return true }); err != nil || requests != 0 {
		t.Fatalf("reachable server: err=%v requests=%d", err, requests)
	}
	err := s.start(context.Background(), func(context.Context) bool { return false })
	if err == nil || requests != 1 || s.proc != nil {
		t.Fatalf("failed unlock: err=%v requests=%d proc=%v", err, requests, s.proc)
	}
}

func TestModelHandsTerminalToBitwardenUnlock(t *testing.T) {
	m, _, _ := loadedModel(t, nil)
	_, cmd := m.Update(bitwardenUnlockRequestMsg{reply: make(chan bitwardenUnlockResult, 1)})
	if cmd == nil {
		t.Fatal("unlock request must start bw unlock")
	}
}
