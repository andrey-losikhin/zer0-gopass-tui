package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"zer0-gopass-tui/internal/gopass"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

type fakeBitwardenSyncer struct {
	path   string
	values []gopass.FieldValue
	err    error
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
