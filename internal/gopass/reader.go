package gopass

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

// manifestPathPrefix — префикс пути gopass, под которым хранятся манифесты
// полей записей (FIELD-CONTRACT.md v1).
const manifestPathPrefix = ".zer0-waypass/v1/manifests/"

// ErrManifestNotFound возвращается, когда по entryPath не удалось прочитать
// манифест полей (запись не существует или является legacy-записью без
// field bundle). Ошибки backend возвращаются отдельно. Отличим от ошибок
// ParseManifest (malformed manifest), чтобы вызывающий код мог предложить
// разные сценарии восстановления.
var ErrManifestNotFound = errors.New("gopass: manifest not found")

// ErrStaleRevision возвращается ResolveField, когда актуальная (свежепрочитанная)
// wire revision записи не совпадает с переданной вызывающим кодом - конкурентное
// изменение манифеста между чтением и explicit reveal значения поля.
var ErrStaleRevision = errors.New("gopass: stale revision")

// ErrFieldNotFound возвращается ResolveField, если fieldID отсутствует
// среди полей свежепрочитанного манифеста.
var ErrFieldNotFound = errors.New("gopass: field not found")

// FieldItem представляет одно поле записи для отображения в TUI.
// Value заполнено только для Visibility == VisibilityPublic; для
// VisibilitySecret значение никогда не читается неявно - только
// через explicit ResolveField.
type FieldItem struct {
	ID         string
	Kind       FieldKind
	Name       string
	Visibility Visibility
	Multiline  bool
	Value      string
}

// FieldSet — набор полей записи вместе с wire revision, вычисленной
// как SHA-256 точных байт манифеста (не путать с внутренним
// Manifest.Revision).
type FieldSet struct {
	BundleID      string
	Revision      string
	Fields        []FieldItem
	BitwardenSync bool
}

// BitwardenSyncFieldName — зарезервированный descriptor opt-in синхронизации.
const BitwardenSyncFieldName = "zer0-waypass.bitwarden-sync"

// Reader читает манифест полей записи и раскрывает значения конкретных
// полей (FIELD-CONTRACT.md v1).
type Reader interface {
	// ReadManifest читает манифест записи entryPath и все её public-значения.
	// Secret-значения не читаются - Value для них остаётся пустой строкой.
	ReadManifest(ctx context.Context, entryPath string) (FieldSet, error)
	// ResolveField выполняет explicit reveal значения одного поля fieldID
	// записи entryPath с повторной проверкой revision и membership поля
	// в манифесте (fail-closed на конкурентное изменение).
	ResolveField(ctx context.Context, entryPath, wireRevision, fieldID string) (string, error)
}

// ExecReader реализует Reader, вызывая бинарь gopass как внешний процесс.
type ExecReader struct{}

// ReadManifest реализует Reader.ReadManifest.
func (r ExecReader) ReadManifest(ctx context.Context, entryPath string) (FieldSet, error) {
	manifest, wireRevision, _, err := r.fetchManifest(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}

	fields := make([]FieldItem, 0, len(manifest.Fields))
	bitwardenSync := false
	for _, f := range manifest.Fields {
		if f.Kind == "custom" && f.Name == BitwardenSyncFieldName && f.Visibility == VisibilityPublic && !f.Multiline {
			value, err := execShow(ctx, fieldValuePath(manifest.BundleID, manifest.Revision, f.ID))
			if err != nil || string(value) != "enabled" {
				return FieldSet{}, fmt.Errorf("gopass: invalid Bitwarden sync marker")
			}
			bitwardenSync = true
			continue
		}
		item := FieldItem{
			ID:         f.ID,
			Kind:       f.Kind,
			Name:       f.Name,
			Visibility: f.Visibility,
			Multiline:  f.Multiline,
		}
		if f.Visibility == VisibilityPublic {
			value, err := execShow(ctx, fieldValuePath(manifest.BundleID, manifest.Revision, f.ID))
			if err != nil {
				return FieldSet{}, fmt.Errorf("gopass: read field value: %w", err)
			}
			if err := ValidFieldValue(value, f.Multiline); err != nil {
				return FieldSet{}, err
			}
			item.Value = string(value)
		}
		fields = append(fields, item)
	}

	return FieldSet{BundleID: manifest.BundleID, Revision: wireRevision, Fields: fields, BitwardenSync: bitwardenSync}, nil
}

// ResolveField реализует Reader.ResolveField.
func (r ExecReader) ResolveField(ctx context.Context, entryPath, wireRevision, fieldID string) (string, error) {
	if !ValidCanonicalID(fieldID) {
		return "", fmt.Errorf("gopass: invalid field id")
	}

	manifest, freshRevision, _, err := r.fetchManifest(ctx, entryPath)
	if err != nil {
		return "", err
	}
	if freshRevision != wireRevision {
		return "", ErrStaleRevision
	}

	var field *Field
	for i := range manifest.Fields {
		if manifest.Fields[i].ID == fieldID {
			field = &manifest.Fields[i]
			break
		}
	}
	if field == nil {
		return "", ErrFieldNotFound
	}

	value, err := execShow(ctx, fieldValuePath(manifest.BundleID, manifest.Revision, fieldID))
	if err != nil {
		return "", fmt.Errorf("gopass: read field value: %w", err)
	}
	if err := ValidFieldValue(value, field.Multiline); err != nil {
		return "", err
	}

	return string(value), nil
}

// fetchManifest читает и валидирует манифест записи entryPath, вычисляя
// wire revision как SHA-256 точных байт манифеста в том виде, как они
// получены от gopass (без какой-либо нормализации или re-marshaling).
// Возвращает также сырые байты манифеста для случаев, когда вызывающему
// коду нужен доступ к ним напрямую.
func (ExecReader) fetchManifest(ctx context.Context, entryPath string) (Manifest, string, []byte, error) {
	manifestID, err := EncodeCanonicalPath(entryPath)
	if err != nil {
		return Manifest{}, "", nil, fmt.Errorf("gopass: invalid entry path: %w", err)
	}

	manifestPath := manifestPathPrefix + manifestID
	exists, err := (execWriterBackend{}).exists(ctx, manifestPath)
	if err != nil {
		return Manifest{}, "", nil, fmt.Errorf("gopass: check manifest: %w", err)
	}
	if !exists {
		return Manifest{}, "", nil, ErrManifestNotFound
	}
	raw, err := execShow(ctx, manifestPath)
	if err != nil {
		return Manifest{}, "", nil, fmt.Errorf("gopass: read manifest: %w", err)
	}

	sum := sha256.Sum256(raw)
	wireRevision := base64.RawURLEncoding.EncodeToString(sum[:])

	manifest, err := ParseManifest(raw)
	if err != nil {
		return Manifest{}, "", nil, err
	}

	return manifest, wireRevision, raw, nil
}

// fieldValuePath строит путь gopass к значению поля записи по внутренним
// bundle_id и revision манифеста (Manifest.Revision - не wire revision).
func fieldValuePath(bundleID, revision, fieldID string) string {
	return fmt.Sprintf(".zer0-waypass/v1/%s/%s/%s", bundleID, revision, fieldID)
}
