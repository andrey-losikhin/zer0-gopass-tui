package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"zer0-gopass-tui/internal/gopass"
)

type bitwardenSyncer interface {
	Upsert(context.Context, string, []gopass.FieldValue) error
}

type bitwardenClient struct {
	baseURL   string
	client    *http.Client
	configErr error
}

type bitwardenItem struct {
	ID             string           `json:"id,omitempty"`
	Type           int              `json:"type"`
	Name           string           `json:"name"`
	Notes          string           `json:"notes,omitempty"`
	Favorite       bool             `json:"favorite"`
	Reprompt       int              `json:"reprompt"`
	FolderID       *string          `json:"folderId"`
	OrganizationID *string          `json:"organizationId"`
	CollectionIDs  []string         `json:"collectionIds"`
	Fields         []bitwardenField `json:"fields,omitempty"`
	Login          bitwardenLogin   `json:"login"`
}

type bitwardenField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Type  int    `json:"type"`
}

type bitwardenLogin struct {
	Username string         `json:"username,omitempty"`
	Password string         `json:"password,omitempty"`
	TOTP     string         `json:"totp,omitempty"`
	URIs     []bitwardenURI `json:"uris,omitempty"`
}

type bitwardenURI struct {
	URI   string `json:"uri"`
	Match *int   `json:"match"`
}

func newBitwardenClient() bitwardenSyncer {
	baseURL := os.Getenv("ZER0_GOPASS_BITWARDEN_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8087"
	}
	parsed, err := url.Parse(baseURL)
	if err == nil {
		host := parsed.Hostname()
		loopback := host == "localhost"
		if ip := net.ParseIP(host); ip != nil {
			loopback = ip.IsLoopback()
		}
		if parsed.Scheme != "https" && !(parsed.Scheme == "http" && loopback) {
			err = fmt.Errorf("Bitwarden URL: http разрешён только для localhost, внешний адрес требует https")
		}
	}
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return bitwardenClient{baseURL: strings.TrimRight(baseURL, "/"), client: client, configErr: err}
}

func (b bitwardenClient) Upsert(ctx context.Context, path string, fields []gopass.FieldValue) error {
	if b.configErr != nil {
		return b.configErr
	}
	if err := b.ensureUnlocked(ctx); err != nil {
		return err
	}
	item := mapBitwardenItem(path, fields)
	existing, err := b.find(ctx, path)
	if err != nil {
		return err
	}
	method, endpoint := http.MethodPost, "/object/item"
	body := any(item)
	if existing != nil {
		id, _ := existing["id"].(string)
		method, endpoint = http.MethodPut, "/object/item/"+url.PathEscape(id)
		body = mergeBitwardenItem(existing, item)
	}
	return b.request(ctx, method, endpoint, body, nil)
}

func (b bitwardenClient) ensureUnlocked(ctx context.Context) error {
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Template struct {
				Status string `json:"status"`
			} `json:"template"`
		} `json:"data"`
	}
	if err := b.request(ctx, http.MethodGet, "/status", nil, &response); err != nil {
		return err
	}
	if !response.Success || response.Data.Template.Status != "unlocked" {
		return fmt.Errorf("Bitwarden vault заблокирован: остановите bw serve, выполните `export BW_SESSION=\"$(bw unlock --raw)\"` и запустите bw serve снова")
	}
	return nil
}

func (b bitwardenClient) find(ctx context.Context, path string) (map[string]any, error) {
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	endpoint := "/list/object/items"
	if err := b.request(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}
	items := []map[string]any{}
	if err := json.Unmarshal(response.Data, &items); err != nil {
		var nested struct {
			Data []map[string]any `json:"data"`
		}
		if nestedErr := json.Unmarshal(response.Data, &nested); nestedErr != nil {
			return nil, fmt.Errorf("Bitwarden: invalid item list")
		}
		items = nested.Data
	}
	for _, item := range items {
		if fields, ok := item["fields"].([]any); ok {
			for _, raw := range fields {
				field, _ := raw.(map[string]any)
				if field["name"] == "zer0-gopass-path" && field["value"] == path {
					return item, nil
				}
			}
		}
	}
	return nil, nil
}

func (b bitwardenClient) request(ctx context.Context, method, endpoint string, body, target any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("Bitwarden: encode item: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, b.baseURL+endpoint, reader)
	if err != nil {
		return fmt.Errorf("Bitwarden: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("Bitwarden недоступен: запустите `bw unlock`, затем `bw serve`: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return fmt.Errorf("Bitwarden вернул HTTP %d", response.StatusCode)
	}
	if target != nil {
		if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(target); err != nil {
			return fmt.Errorf("Bitwarden: invalid response: %w", err)
		}
	}
	return nil
}
