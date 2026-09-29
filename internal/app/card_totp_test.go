package app

import (
	"context"
	"strings"
	"testing"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestRevealedTOTPShowsCodeAndTicks(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" // base32("12345678901234567890"), RFC 6238 seed
	c := newCard(context.Background(), &fakeReader{}, &fakeWriter{}, "entry")
	c.loading = false
	c.set = gopass.FieldSet{Revision: "wire", Fields: []gopass.FieldItem{{ID: "totp-id", Kind: "totp_secret", Name: "TOTP secret", Visibility: gopass.VisibilitySecret}}}
	c, cmd, _ := c.update(revealMsg{fieldID: "totp-id", value: secret})
	if cmd == nil {
		t.Fatal("TOTP reveal did not schedule refresh")
	}
	view := c.view()
	if !strings.Contains(view, "код ") || strings.Contains(view, secret) {
		t.Fatalf("view = %q", view)
	}
	c.revealed = map[string]string{}
	if _, cmd, _ = c.update(totpTickMsg{}); cmd != nil {
		t.Fatal("ticker continues after secret was hidden")
	}
}
