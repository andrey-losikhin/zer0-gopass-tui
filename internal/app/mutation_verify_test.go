package app

import (
	"context"
	"testing"

	"zer0-gopass-tui/internal/gopass"
)

func TestCardMutationReloadsReaderAndList(t *testing.T) {
	vault := newFakeVault()
	set, _ := vault.CreateBundle(context.Background(), "entry", allStandardFields()[:1])
	m := NewModel(context.Background(), vault, vault, vault)
	m.mode = modeCard
	m.card = newCard(m.ctx, vault, vault, "entry")
	m.card.loading = true

	updated, verify := m.Update(mutationMsg{set: set})
	m = updated.(Model)
	if verify == nil {
		t.Fatal("successful mutation did not schedule backend reload")
	}
	updated, _ = m.Update(verify())
	m = updated.(Model)
	if m.card.loading || len(m.card.set.Fields) != 1 || vault.reads != 1 || vault.lists != 1 {
		t.Fatalf("card=%#v reads=%d lists=%d", m.card, vault.reads, vault.lists)
	}
}

func TestCardMutationRejectsEntryMissingFromList(t *testing.T) {
	vault := newFakeVault()
	set, _ := vault.CreateBundle(context.Background(), "entry", allStandardFields()[:1])
	delete(vault.sets, "entry")
	m := NewModel(context.Background(), vault, &fakeReader{sets: map[string]gopass.FieldSet{"entry": set}}, vault)
	m.mode = modeCard
	m.card = newCard(m.ctx, m.reader, vault, "entry")
	m.card.loading = true

	updated, verify := m.Update(mutationMsg{set: set})
	m = updated.(Model)
	updated, _ = m.Update(verify())
	m = updated.(Model)
	if m.card.err == nil || m.card.loading {
		t.Fatalf("missing list entry accepted: %#v", m.card)
	}
}
