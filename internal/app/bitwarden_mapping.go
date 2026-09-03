package app

import (
	"encoding/json"
	"slices"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

const bitwardenManagedField = "zer0-gopass-managed"

func mergeBitwardenItem(merged map[string]any, desired bitwardenItem) map[string]any {
	merged["name"] = desired.Name
	oldManaged := managedFields(merged["fields"])
	newManaged := managedFields(desired.Fields)
	if newManaged["notes"] {
		merged["notes"] = desired.Notes
	} else if oldManaged["notes"] {
		merged["notes"] = ""
	}
	login, _ := merged["login"].(map[string]any)
	if login == nil {
		login = make(map[string]any)
	}
	mergeLoginValue(login, "username", desired.Login.Username, oldManaged, newManaged)
	mergeLoginValue(login, "password", desired.Login.Password, oldManaged, newManaged)
	mergeLoginValue(login, "totp", desired.Login.TOTP, oldManaged, newManaged)
	if newManaged["uris"] {
		login["uris"] = desired.Login.URIs
	} else if oldManaged["uris"] {
		login["uris"] = []any{}
	}
	merged["login"] = login
	desiredNames := make(map[string]bool, len(desired.Fields))
	for _, field := range desired.Fields {
		desiredNames[field.Name] = true
	}
	kept := make([]any, 0)
	if fields, ok := merged["fields"].([]any); ok {
		for _, raw := range fields {
			field, _ := raw.(map[string]any)
			name, _ := field["name"].(string)
			if !desiredNames[name] && !oldManaged["field:"+name] {
				kept = append(kept, raw)
			}
		}
	}
	for _, field := range desired.Fields {
		kept = append(kept, field)
	}
	merged["fields"] = kept
	return merged
}

func mergeLoginValue(login map[string]any, key, value string, oldManaged, newManaged map[string]bool) {
	if newManaged[key] {
		login[key] = value
	} else if oldManaged[key] {
		login[key] = ""
	}
}

func managedFields(raw any) map[string]bool {
	managed := make(map[string]bool)
	var fields []any
	switch value := raw.(type) {
	case []any:
		fields = value
	case []bitwardenField:
		for _, field := range value {
			fields = append(fields, field)
		}
	}
	for _, rawField := range fields {
		var name, value string
		switch field := rawField.(type) {
		case map[string]any:
			name, _ = field["name"].(string)
			value, _ = field["value"].(string)
		case bitwardenField:
			name, value = field.Name, field.Value
		}
		if name == bitwardenManagedField {
			var names []string
			if json.Unmarshal([]byte(value), &names) == nil {
				for _, item := range names {
					managed[item] = true
				}
			}
		}
	}
	return managed
}

func mapBitwardenItem(path string, values []gopass.FieldValue) bitwardenItem {
	item := bitwardenItem{Type: 1, Name: path, CollectionIDs: []string{}, Fields: []bitwardenField{{Name: "zer0-gopass-path", Value: path, Type: 0}}}
	managed := make([]string, 0, len(values))
	for _, value := range values {
		switch value.Kind {
		case "username":
			item.Login.Username = value.Value
			managed = append(managed, "username")
		case "password":
			item.Login.Password = value.Value
			managed = append(managed, "password")
		case "url":
			item.Login.URIs = append(item.Login.URIs, bitwardenURI{URI: value.Value})
			if !slices.Contains(managed, "uris") {
				managed = append(managed, "uris")
			}
		case "totp_secret":
			item.Login.TOTP = value.Value
			managed = append(managed, "totp")
		case "notes":
			item.Notes = value.Value
			managed = append(managed, "notes")
		default:
			typeID := 0
			if value.Visibility == gopass.VisibilitySecret {
				typeID = 1
			}
			item.Fields = append(item.Fields, bitwardenField{Name: value.Name, Value: value.Value, Type: typeID})
			managed = append(managed, "field:"+value.Name)
		}
	}
	raw, _ := json.Marshal(managed)
	item.Fields = append(item.Fields, bitwardenField{Name: bitwardenManagedField, Value: string(raw), Type: 1})
	return item
}
