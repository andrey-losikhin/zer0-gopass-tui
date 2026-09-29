package gopass

import (
	"context"
	"fmt"
	"strings"
)

// legacyAliases сопоставляет нормализованные имена ключей legacy-записей
// стандартным kind. Имена, совпадающие с kind, обрабатываются без таблицы.
var legacyAliases = map[string]FieldKind{
	"login": "username", "user": "username", "user_name": "username",
	"website": "url", "site": "url", "uri": "url", "link": "url",
	"e_mail": "email", "mail": "email",
	"note": "notes", "comment": "notes", "comments": "notes",
	"totp": "totp_secret", "otp": "totp_secret", "otpauth": "totp_secret",
	"apikey": "api_key", "api": "api_key",
}

// ReadLegacy расшифровывает legacy-запись по явному запросу миграции и
// разбирает её в предзаполненные поля. Результат не сохраняется.
func (ExecReader) ReadLegacy(ctx context.Context, entryPath string) ([]FieldValue, error) {
	if err := validateCanonicalPath(entryPath); err != nil {
		return nil, err
	}
	raw, err := execShow(ctx, entryPath)
	if err != nil {
		return nil, fmt.Errorf("gopass: read legacy entry: %w", err)
	}
	return ParseLegacy(raw), nil
}

// ParseLegacy переводит содержимое обычной записи gopass в поля: первая строка —
// password, `key: value` — стандартный kind по имени/алиасу или secret custom,
// `otpauth://` — totp_secret. Нераспознанные строки и дубликаты собираются в
// notes, чтобы при миграции ничего не потерялось.
func ParseLegacy(raw []byte) []FieldValue {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	fields := make([]FieldValue, 0, len(lines))
	usedKinds := make(map[FieldKind]bool)
	usedNames := make(map[string]bool)
	var notes []string
	addStandard := func(kind FieldKind, value string) bool {
		if usedKinds[kind] || value == "" {
			return false
		}
		field, _ := StandardField(kind)
		field.Value = value
		fields = append(fields, field)
		usedKinds[kind], usedNames[field.Name] = true, true
		return true
	}
	if len(lines) > 0 && lines[0] != "" {
		addStandard("password", lines[0])
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			notes = append(notes, line)
			continue
		case trimmed == compatibilityMarker:
			continue
		case strings.HasPrefix(trimmed, "otpauth://"):
			if !addStandard("totp_secret", trimmed) {
				notes = append(notes, line)
			}
			continue
		case strings.HasPrefix(trimmed, "http://"), strings.HasPrefix(trimmed, "https://"):
			if !addStandard("url", trimmed) {
				notes = append(notes, line)
			}
			continue
		}
		name, value, ok := strings.Cut(trimmed, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" || value == "" || len(fields) >= manifestMaxFields-2 {
			notes = append(notes, line)
			continue
		}
		key := strings.ToLower(strings.NewReplacer(" ", "_", "-", "_").Replace(name))
		kind, alias := legacyAliases[key]
		if !alias {
			kind = FieldKind(key)
		}
		if kind == "notes" {
			notes = append(notes, value)
			continue
		}
		if _, standard := standardFields[kind]; standard {
			if !addStandard(kind, value) {
				notes = append(notes, line)
			}
			continue
		}
		if !validDisplayName(name) || usedNames[name] || name == BitwardenSyncFieldName {
			notes = append(notes, line)
			continue
		}
		fields = append(fields, FieldValue{Kind: fieldKindCustom, Name: name, Visibility: VisibilitySecret, Value: value})
		usedNames[name] = true
	}
	for len(notes) > 0 && strings.TrimSpace(notes[len(notes)-1]) == "" {
		notes = notes[:len(notes)-1]
	}
	for len(notes) > 0 && strings.TrimSpace(notes[0]) == "" {
		notes = notes[1:]
	}
	if len(notes) > 0 {
		addStandard("notes", strings.Join(notes, "\n"))
	}
	return fields
}
