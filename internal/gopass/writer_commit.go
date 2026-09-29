package gopass

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func (w ExecWriter) loadCurrent(ctx context.Context, entryPath, wireRevision string) (Manifest, string, []byte, []FieldValue, error) {
	m, path, raw, err := w.loadManifest(ctx, entryPath, wireRevision)
	if err != nil {
		return Manifest{}, "", nil, nil, err
	}
	values := make([]FieldValue, len(m.Fields))
	for i, f := range m.Fields {
		value, err := w.store().show(ctx, fieldValuePath(m.BundleID, m.Revision, f.ID))
		if err != nil {
			return Manifest{}, "", nil, nil, fmt.Errorf("gopass: read field value: %w", err)
		}
		if err := ValidFieldValue(value, f.Multiline); err != nil {
			return Manifest{}, "", nil, nil, err
		}
		values[i] = FieldValue{Kind: f.Kind, Name: f.Name, Visibility: f.Visibility, Multiline: f.Multiline, Value: string(value)}
	}
	return m, path, raw, values, nil
}

func (w ExecWriter) loadManifest(ctx context.Context, entryPath, wireRevision string) (Manifest, string, []byte, error) {
	path, err := encodedManifestPath(entryPath)
	if err != nil {
		return Manifest{}, "", nil, err
	}
	raw, err := w.store().show(ctx, path)
	if err != nil {
		return Manifest{}, "", nil, fmt.Errorf("%w: %v", ErrManifestNotFound, err)
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return Manifest{}, "", nil, err
	}
	if wireRevisionOf(raw) != wireRevision {
		return Manifest{}, "", nil, ErrStaleRevision
	}
	return m, path, raw, nil
}

// commit записывает новую revision и переключает manifest. previousRaw — точные
// байты текущего manifest (nil при создании): они нужны для отката, если
// записанный manifest не подтвердился.
func (w ExecWriter) commit(ctx context.Context, manifestPath, expectedRevision string, base Manifest, previousRaw []byte, values []FieldValue) (FieldSet, error) {
	revision, err := GenerateID()
	if err != nil {
		return FieldSet{}, err
	}
	next := Manifest{Format: manifestFormat, BundleID: base.BundleID, Revision: revision, Fields: make([]Field, len(values))}
	for i, value := range values {
		id, err := GenerateID()
		if err != nil {
			return FieldSet{}, err
		}
		next.Fields[i] = Field{ID: id, Kind: value.Kind, Name: value.Name, Visibility: value.Visibility, Multiline: value.Multiline}
		if err := ValidFieldValue([]byte(value.Value), value.Multiline); err != nil {
			return FieldSet{}, err
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return FieldSet{}, fmt.Errorf("gopass: encode manifest: %w", err)
	}
	if _, err := ParseManifest(raw); err != nil {
		return FieldSet{}, err
	}

	newDir := revisionDir(next.BundleID, next.Revision)
	wroteAny := false
	rollbackError := func(cause error) error {
		if !wroteAny {
			return cause
		}
		if err := w.store().removeTree(ctx, newDir); err != nil {
			return fmt.Errorf("%w; cleanup of new revision failed", cause)
		}
		return cause
	}
	for i, value := range values {
		path := fieldValuePath(next.BundleID, next.Revision, next.Fields[i].ID)
		if err := w.store().write(ctx, path, []byte(value.Value)); err != nil {
			return FieldSet{}, rollbackError(err)
		}
		wroteAny = true
		got, err := w.store().show(ctx, path)
		if err != nil || string(got) != value.Value {
			return FieldSet{}, rollbackError(fmt.Errorf("gopass: value verification failed for field %s", next.Fields[i].ID))
		}
	}
	// Повторная проверка непосредственно перед заменой manifest сужает окно
	// гонки до одного вызова gopass; атомарного compare-and-swap у gopass нет.
	if expectedRevision != "" {
		fresh, err := w.store().show(ctx, manifestPath)
		if err != nil || wireRevisionOf(fresh) != expectedRevision {
			return FieldSet{}, rollbackError(ErrStaleRevision)
		}
	} else {
		exists, err := w.store().exists(ctx, manifestPath)
		if err != nil {
			return FieldSet{}, rollbackError(err)
		}
		if exists {
			return FieldSet{}, rollbackError(errManifestExists)
		}
	}
	if err := w.store().write(ctx, manifestPath, raw); err != nil {
		return FieldSet{}, rollbackError(err)
	}
	confirmed, err := w.store().show(ctx, manifestPath)
	if err != nil || !bytes.Equal(confirmed, raw) {
		return FieldSet{}, w.recoverManifest(ctx, manifestPath, previousRaw, confirmed, err == nil, newDir)
	}
	set := fieldSetFrom(next, values, wireRevisionOf(raw))
	if previousRaw != nil && base.Revision != "" {
		if err := w.store().removeTree(ctx, revisionDir(base.BundleID, base.Revision)); err != nil {
			return set, &CleanupError{Failed: 1}
		}
	}
	return set, nil
}

// recoverManifest выполняет best-effort откат после неподтверждённой записи
// manifest. Старая revision ещё не удалена, поэтому возврат previousRaw
// восстанавливает рабочее состояние. Валидный чужой manifest означает
// конкурентную запись: его не трогаем, новая revision остаётся для gc.
func (w ExecWriter) recoverManifest(ctx context.Context, manifestPath string, previousRaw, confirmed []byte, readOK bool, newDir string) error {
	if readOK {
		if previousRaw != nil && bytes.Equal(confirmed, previousRaw) {
			if err := w.store().removeTree(ctx, newDir); err != nil {
				return fmt.Errorf("gopass: manifest write was not applied; cleanup of new revision failed")
			}
			return fmt.Errorf("gopass: manifest write was not applied")
		}
		if _, err := ParseManifest(confirmed); err == nil {
			return fmt.Errorf("%w: manifest changed concurrently after write", ErrStaleRevision)
		}
	}
	var restoreErr error
	if previousRaw != nil {
		restoreErr = w.store().write(ctx, manifestPath, previousRaw)
		if restoreErr == nil {
			restored, err := w.store().show(ctx, manifestPath)
			if err != nil || !bytes.Equal(restored, previousRaw) {
				restoreErr = fmt.Errorf("restored manifest mismatch")
			}
		}
	} else {
		restoreErr = w.store().remove(ctx, manifestPath)
	}
	if restoreErr != nil {
		return fmt.Errorf("gopass: manifest verification failed; rollback failed, both revisions kept")
	}
	if err := w.store().removeTree(ctx, newDir); err != nil {
		return fmt.Errorf("gopass: manifest verification failed; previous manifest restored, new revision not cleaned")
	}
	return fmt.Errorf("gopass: manifest verification failed; previous manifest restored")
}

func (w ExecWriter) cleanup(ctx context.Context, paths []string) int {
	failed := 0
	for _, path := range paths {
		if err := w.store().remove(ctx, path); err != nil {
			failed++
		}
	}
	return failed
}

func oldValuePaths(m Manifest) []string {
	paths := make([]string, len(m.Fields))
	for i, field := range m.Fields {
		paths[i] = fieldValuePath(m.BundleID, m.Revision, field.ID)
	}
	return paths
}

func revisionDir(bundleID, revision string) string {
	return fmt.Sprintf(".zer0-waypass/v1/%s/%s", bundleID, revision)
}

func bundleDir(bundleID string) string {
	return ".zer0-waypass/v1/" + bundleID
}

func wireRevisionOf(raw []byte) string {
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func fieldSetFrom(m Manifest, values []FieldValue, revision string) FieldSet {
	items := make([]FieldItem, len(m.Fields))
	for i, field := range m.Fields {
		items[i] = FieldItem{ID: field.ID, Kind: field.Kind, Name: field.Name, Visibility: field.Visibility, Multiline: field.Multiline}
		if field.Visibility == VisibilityPublic {
			items[i].Value = values[i].Value
		}
	}
	return FieldSet{BundleID: m.BundleID, Revision: revision, Fields: items}
}
