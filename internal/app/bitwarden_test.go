package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

type fakeBitwardenSyncer struct {
	path     string
	values   []gopass.FieldValue
	err      error
	match    *bitwardenMatch
	finds    []string
	deleted  []string
	relinked []string
}

func (f *fakeBitwardenSyncer) Find(_ context.Context, path string) (bitwardenMatch, bool, error) {
	f.finds = append(f.finds, path)
	if f.match == nil {
		return bitwardenMatch{}, false, f.err
	}
	return *f.match, true, f.err
}

func (f *fakeBitwardenSyncer) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	return f.err
}

func (f *fakeBitwardenSyncer) Relink(_ context.Context, oldPath, newPath string) error {
	f.relinked = append(f.relinked, oldPath+" -> "+newPath)
	return f.err
}

func (f *fakeBitwardenSyncer) Upsert(_ context.Context, path string, values []gopass.FieldValue) error {
	f.path, f.values = path, append([]gopass.FieldValue(nil), values...)
	return f.err
}

func TestBitwardenCreatesMappedLogin(t *testing.T) {
	var created bitwardenItem
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/status":
			return jsonResponse(`{"success":true,"data":{"template":{"status":"unlocked"}}}`), nil
		case "/list/object/items":
			return jsonResponse(`{"data":[]}`), nil
		case "/object/item":
			if r.Method != http.MethodPost {
				t.Fatalf("method=%s", r.Method)
			}
			_ = json.NewDecoder(r.Body).Decode(&created)
			return jsonResponse(`{"id":"new-id"}`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
	})
	client := bitwardenClient{baseURL: "http://bitwarden.test", client: &http.Client{Transport: transport}}
	values := []gopass.FieldValue{
		{Kind: "username", Name: "Username", Value: "alice"},
		{Kind: "password", Name: "Password", Visibility: gopass.VisibilitySecret, Value: "secret"},
		{Kind: "url", Name: "URL", Value: "https://example.test"},
	}
	if err := client.Upsert(context.Background(), "example/account", values); err != nil {
		t.Fatal(err)
	}
	if created.Name != "example/account" || created.Login.Username != "alice" || created.Login.Password != "secret" || len(created.Login.URIs) != 1 {
		t.Fatalf("created item=%#v", created)
	}
}

func TestBitwardenUpdatesItemWithMatchingStablePath(t *testing.T) {
	updated := false
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/status" {
			return jsonResponse(`{"success":true,"data":{"template":{"status":"unlocked"}}}`), nil
		}
		if r.URL.Path == "/list/object/items" {
			return jsonResponse(`{"data":{"data":[{"id":"item-id","folderId":"folder-id","favorite":true,"fields":[{"name":"zer0-gopass-path","value":"entry"},{"name":"Bitwarden only","value":"keep"}],"login":{"fido2Credentials":[{"credentialId":"keep"}]}}]}}`), nil
		}
		if r.Method == http.MethodPut && r.URL.Path == "/object/item/item-id" {
			updated = true
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			login, _ := body["login"].(map[string]any)
			if body["folderId"] != "folder-id" || body["favorite"] != true || login["fido2Credentials"] == nil {
				t.Fatalf("Bitwarden-only properties lost: %#v", body)
			}
			return jsonResponse(`{"id":"item-id"}`), nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	client := bitwardenClient{baseURL: "http://bitwarden.test", client: &http.Client{Transport: transport}}
	if err := client.Upsert(context.Background(), "entry", nil); err != nil || !updated {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
}

func TestBitwardenReportsLockedVaultBeforeSendingSecrets(t *testing.T) {
	requests := 0
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		return jsonResponse(`{"success":true,"data":{"template":{"status":"locked"}}}`), nil
	})
	client := bitwardenClient{baseURL: "http://bitwarden.test", client: &http.Client{Transport: transport}}
	err := client.Upsert(context.Background(), "entry", []gopass.FieldValue{{Kind: "password", Value: "must-not-be-sent"}})
	if err == nil || !strings.Contains(err.Error(), "заблокирован") || requests != 1 || strings.Contains(err.Error(), "must-not-be-sent") {
		t.Fatalf("requests=%d err=%v", requests, err)
	}
}

func TestBitwardenMergeClearsOnlyPreviouslyManagedValues(t *testing.T) {
	existing := map[string]any{
		"login": map[string]any{"password": "old-secret", "username": "bitwarden-only"},
		"fields": []any{
			map[string]any{"name": "API", "value": "old-api"},
			map[string]any{"name": "Bitwarden only", "value": "keep"},
			map[string]any{"name": bitwardenManagedField, "value": `["password","field:API"]`},
		},
	}
	merged := mergeBitwardenItem(existing, mapBitwardenItem("entry", nil))
	login := merged["login"].(map[string]any)
	if login["password"] != "" || login["username"] != "bitwarden-only" {
		t.Fatalf("login merge=%#v", login)
	}
	for _, raw := range merged["fields"].([]any) {
		field, ok := raw.(map[string]any)
		if ok && field["name"] == "API" {
			t.Fatal("removed managed custom field was retained")
		}
	}
}

func TestCreateCheckboxAddsEncryptedSyncMarker(t *testing.T) {
	w := &fakeWriter{set: testSet()}
	c := newCreate(context.Background(), w, "")
	c.path.SetValue("entry")
	c.fields[0].Value = "secret"
	c.editing = false
	c, _, _ = c.update(keyRunes("b"))
	c, cmd, _ := c.update(keyCtrlS())
	_ = cmd()
	last := w.createdFields[len(w.createdFields)-1]
	if !c.syncBitwarden || last.Name != gopass.BitwardenSyncFieldName || last.Visibility != gopass.VisibilityPublic {
		t.Fatalf("sync marker=%#v enabled=%v", last, c.syncBitwarden)
	}
}

func TestUncheckedCreateDoesNotAddSyncMarker(t *testing.T) {
	w := &fakeWriter{set: testSet()}
	c := newCreate(context.Background(), w, "")
	c.path.SetValue("entry")
	c.fields[0].Value = "secret"
	c.editing = false
	c, cmd, _ := c.update(keyCtrlS())
	_ = cmd()
	for _, field := range w.createdFields {
		if field.Name == gopass.BitwardenSyncFieldName {
			t.Fatal("unchecked form added sync marker")
		}
	}
}

func TestCheckedCreateEndToEndSyncsResolvedSecret(t *testing.T) {
	vault := newFakeVault()
	syncer := &fakeBitwardenSyncer{}
	m := NewModel(context.Background(), vault, vault, vault)
	m.bitwarden = syncer
	updated, _ := m.Update(entriesLoadedMsg{})
	m = updated.(Model)
	updated, _ = m.Update(keyRunes("n"))
	m = updated.(Model)
	m.create.path.SetValue("entry")
	m.create.fields[0].Value = "end-to-end-secret"
	m.create.syncBitwarden = true
	m.create.editing = false

	updated, save := m.Update(keyCtrlS())
	m = updated.(Model)
	updated, verify := m.Update(save())
	m = updated.(Model)
	updated, sync := m.Update(verify())
	m = updated.(Model)
	if sync == nil {
		t.Fatal("verified checked create did not schedule sync")
	}
	updated, _ = m.Update(sync())
	m = updated.(Model)
	if syncer.path != "entry" || len(syncer.values) != 1 || syncer.values[0].Value != "end-to-end-secret" {
		t.Fatalf("sync payload=%#v", syncer)
	}
	if m.notice == nil || m.card.err != nil {
		t.Fatalf("notice=%v card error=%v", m.notice, m.card.err)
	}
}

func TestVerifiedCheckedCreateAutomaticallySyncsBitwarden(t *testing.T) {
	m, reader, _ := loadedModel(t, nil)
	syncer := &fakeBitwardenSyncer{}
	m.bitwarden = syncer
	m.mode = modeCreate
	m.create = newCreate(m.ctx, m.writer, "")
	m.create.syncBitwarden = true
	reader.sets["entry"] = testSet()

	updated, cmd := m.Update(createdVerifiedMsg{path: "entry", entries: []gopass.Entry{{Path: "entry"}}, set: testSet()})
	m = updated.(Model)
	if cmd == nil || m.mode != modeCard {
		t.Fatalf("sync not scheduled: mode=%v cmd=%v", m.mode, cmd)
	}
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if syncer.path != "entry" || len(syncer.values) != 2 || m.notice == nil {
		t.Fatalf("syncer=%#v notice=%v", syncer, m.notice)
	}
}

func keyCtrlS() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlS} }

func TestBitwardenFindUsesSearchBeforeFullList(t *testing.T) {
	var queries []string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/status":
			return jsonResponse(`{"success":true,"data":{"template":{"status":"unlocked"}}}`), nil
		case "/list/object/items":
			queries = append(queries, r.URL.Query().Get("search"))
			return jsonResponse(`{"data":{"data":[{"id":"item-id","name":"work/a","fields":[{"name":"zer0-gopass-path","value":"work/a"}]}]}}`), nil
		}
		return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	client := bitwardenClient{baseURL: "http://bitwarden.test", client: &http.Client{Transport: transport}}
	match, found, err := client.Find(context.Background(), "work/a")
	if err != nil || !found || match.ID != "item-id" {
		t.Fatalf("Find() = %#v, %v, %v", match, found, err)
	}
	if len(queries) != 1 || queries[0] != "work/a" {
		t.Fatalf("list queries = %q, want one search request", queries)
	}
}

func TestBitwardenDeleteUsesItemEndpoint(t *testing.T) {
	var method, path string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/status" {
			return jsonResponse(`{"success":true,"data":{"template":{"status":"unlocked"}}}`), nil
		}
		method, path = r.Method, r.URL.Path
		return jsonResponse(`{"success":true}`), nil
	})
	client := bitwardenClient{baseURL: "http://bitwarden.test", client: &http.Client{Transport: transport}}
	if err := client.Delete(context.Background(), "item-id"); err != nil {
		t.Fatal(err)
	}
	if method != http.MethodDelete || path != "/object/item/item-id" {
		t.Fatalf("request = %s %s", method, path)
	}
}

func TestEntryDeleteAsksBeforeDeletingBitwardenItem(t *testing.T) {
	m, r, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	set := testSet()
	set.BitwardenSync = true
	r.sets["work/account"] = set
	syncer := &fakeBitwardenSyncer{match: &bitwardenMatch{ID: "bw-1", Name: "work/account"}}
	m.bitwarden = syncer
	updated, cmd := m.Update(keyRunes("d"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	updated, cmd = m.Update(keyRunes("y"))
	m = updated.(Model)
	updated, cmd = m.Update(cmd())
	m = updated.(Model)
	for _, msg := range drainBatch(cmd) {
		updated, _ = m.Update(msg)
		m = updated.(Model)
	}
	if len(syncer.finds) != 1 || m.bwDelete == nil || len(syncer.deleted) != 0 {
		t.Fatalf("finds=%v prompt=%v deleted=%v", syncer.finds, m.bwDelete, syncer.deleted)
	}
	if !strings.Contains(m.View(), "bw-1") {
		t.Fatal("prompt does not show what will be deleted")
	}
	updated, cmd = m.Update(keyRunes("y"))
	m = updated.(Model)
	updated, _ = m.Update(cmd())
	m = updated.(Model)
	if len(syncer.deleted) != 1 || syncer.deleted[0] != "bw-1" {
		t.Fatalf("deleted=%v", syncer.deleted)
	}
}

func TestDeclinedBitwardenDeleteKeepsItem(t *testing.T) {
	m, _, _ := loadedModel(t, nil)
	syncer := &fakeBitwardenSyncer{}
	m.bitwarden = syncer
	updated, _ := m.Update(bitwardenFoundMsg{path: "entry", match: bitwardenMatch{ID: "bw-1", Name: "entry"}, found: true})
	m = updated.(Model)
	updated, cmd := m.Update(keyRunes("n"))
	m = updated.(Model)
	if cmd != nil || len(syncer.deleted) != 0 || m.bwDelete != nil {
		t.Fatal("declined prompt deleted Bitwarden item")
	}
}

func TestUnsettingSyncFlagLooksUpBitwardenItem(t *testing.T) {
	m, r, _ := loadedModel(t, []gopass.Entry{{Path: "work/account"}})
	syncer := &fakeBitwardenSyncer{}
	m.bitwarden = syncer
	set := testSet()
	set.BitwardenSync = true
	r.sets["work/account"] = testSet()
	m.mode = modeCard
	m.card = newCard(m.ctx, r, m.writer, "work/account")
	m.card.loading = false
	m.card.set = set
	m.card.mode = cardEditAll
	m.card.editor = newCreate(m.ctx, m.writer, "work/account")
	m.card.editor.syncBitwarden = false
	updated, verify := m.Update(createdMsg{path: "work/account"})
	m = updated.(Model)
	updated, find := m.Update(verify())
	m = updated.(Model)
	if find == nil {
		t.Fatal("unsetting sync did not look up Bitwarden item")
	}
	_ = find()
	if len(syncer.finds) != 1 {
		t.Fatalf("finds=%v", syncer.finds)
	}
}

// drainBatch раскрывает tea.Batch в список сообщений для синхронного теста.
func drainBatch(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, drainBatch(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}
