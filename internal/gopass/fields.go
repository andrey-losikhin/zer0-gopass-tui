package gopass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Visibility определяет, показывается ли значение поля в открытом виде
// или маскируется как секрет (см. FIELD-CONTRACT.md v1).
type Visibility string

const (
	VisibilityPublic Visibility = "public"
	VisibilitySecret Visibility = "secret"
)

// FieldKind — тип поля записи: один из стандартных kind закрытой таблицы
// или "custom" для произвольных полей, заданных пользователем в TUI.
type FieldKind string

const fieldKindCustom FieldKind = "custom"

// fieldSpec описывает атрибуты стандартного kind, зафиксированные
// в закрытой таблице FIELD-CONTRACT.md v1: имя по умолчанию,
// видимость и признак многострочности.
type fieldSpec struct {
	Name       string
	Visibility Visibility
	Multiline  bool
}

// standardFields — закрытая таблица стандартных kind. Ровно эти 20 записей,
// никакие другие kind (кроме "custom") не допускаются. Для стандартных kind
// visibility и multiline в манифесте обязаны точно совпадать с этой таблицей.
var standardFields = map[FieldKind]fieldSpec{
	"password":       {"Password", VisibilitySecret, false},
	"username":       {"Username", VisibilityPublic, false},
	"url":            {"URL", VisibilityPublic, false},
	"email":          {"Email", VisibilityPublic, false},
	"notes":          {"Notes", VisibilityPublic, true},
	"host":           {"Host", VisibilityPublic, false},
	"port":           {"Port", VisibilityPublic, false},
	"database":       {"Database", VisibilityPublic, false},
	"engine":         {"Engine", VisibilityPublic, false},
	"dsn":            {"DSN", VisibilityPublic, false},
	"client_id":      {"Client ID", VisibilityPublic, false},
	"jump_host":      {"Jump host", VisibilityPublic, false},
	"api_key":        {"API key", VisibilitySecret, false},
	"token":          {"Token", VisibilitySecret, false},
	"client_secret":  {"Client secret", VisibilitySecret, false},
	"private_key":    {"Private key", VisibilitySecret, true},
	"passphrase":     {"Passphrase", VisibilitySecret, false},
	"sudo_password":  {"Sudo password", VisibilitySecret, false},
	"totp_secret":    {"TOTP secret", VisibilitySecret, false},
	"recovery_codes": {"Recovery codes", VisibilitySecret, true},
}

// Field описывает одно поле записи в манифесте (FIELD-CONTRACT.md v1).
type Field struct {
	ID         string     `json:"id"`
	Kind       FieldKind  `json:"kind"`
	Name       string     `json:"name"`
	Visibility Visibility `json:"visibility"`
	Multiline  bool       `json:"multiline"`
}

// Manifest описывает набор полей записи gopass согласно FIELD-CONTRACT.md v1.
type Manifest struct {
	Format   string  `json:"format"`
	BundleID string  `json:"bundle_id"`
	Revision string  `json:"revision"`
	Fields   []Field `json:"fields"`
}

const manifestFormat = "zer0-waypass/fields-v1"
const manifestMaxBytes = 64 * 1024 // 64 KiB
const manifestMinFields = 1
const manifestMaxFields = 64

// ParseManifest разбирает и полностью fail-closed валидирует манифест полей
// (FIELD-CONTRACT.md v1). При любом нарушении контракта - размер, кодировка,
// неизвестные или дублирующиеся JSON-ключи, формат, canonical ID, количество
// полей, состав полей, соответствие стандартному kind, имя поля - возвращается
// ошибка и пустой Manifest; частично распарсенный манифест никогда не
// возвращается. Текст ошибок не содержит исходных байт манифеста.
func ParseManifest(raw []byte) (Manifest, error) {
	if len(raw) > manifestMaxBytes {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: exceeds size limit")
	}
	if !utf8Valid(raw) {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: invalid UTF-8")
	}
	if err := checkDuplicateKeys(raw); err != nil {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: duplicate key")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: malformed JSON")
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: trailing data")
	}

	if m.Format != manifestFormat {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: unsupported format")
	}
	if !ValidCanonicalID(m.BundleID) {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: invalid bundle id")
	}
	if !ValidCanonicalID(m.Revision) {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: invalid revision")
	}
	if len(m.Fields) < manifestMinFields || len(m.Fields) > manifestMaxFields {
		return Manifest{}, fmt.Errorf("gopass: invalid manifest: invalid field count")
	}

	if err := validateFields(m.Fields); err != nil {
		return Manifest{}, err
	}

	return m, nil
}

// validateFields проверяет состав полей манифеста: canonical ID, уникальность
// ID и имён, принадлежность kind закрытой таблице (или "custom") и точное
// соответствие visibility/multiline таблице для стандартных kind.
func validateFields(fields []Field) error {
	seenIDs := make(map[string]bool, len(fields))
	seenNames := make(map[string]bool, len(fields))
	seenKinds := make(map[FieldKind]bool, len(fields))

	for _, f := range fields {
		if !ValidCanonicalID(f.ID) {
			return fmt.Errorf("gopass: invalid manifest: invalid field id")
		}
		if seenIDs[f.ID] {
			return fmt.Errorf("gopass: invalid manifest: duplicate field id")
		}
		seenIDs[f.ID] = true

		if !validDisplayName(f.Name) {
			return fmt.Errorf("gopass: invalid manifest: invalid field name")
		}
		if seenNames[f.Name] {
			return fmt.Errorf("gopass: invalid manifest: duplicate field name")
		}
		seenNames[f.Name] = true

		if f.Kind == fieldKindCustom {
			if f.Visibility != VisibilityPublic && f.Visibility != VisibilitySecret {
				return fmt.Errorf("gopass: invalid manifest: invalid field visibility")
			}
			continue
		}

		spec, ok := standardFields[f.Kind]
		if !ok {
			return fmt.Errorf("gopass: invalid manifest: unknown field kind")
		}
		if seenKinds[f.Kind] {
			return fmt.Errorf("gopass: invalid manifest: duplicate standard field kind")
		}
		seenKinds[f.Kind] = true
		if f.Visibility != spec.Visibility || f.Multiline != spec.Multiline {
			return fmt.Errorf("gopass: invalid manifest: field metadata mismatch")
		}
	}

	return nil
}
