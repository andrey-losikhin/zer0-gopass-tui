package gopass

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

func (w ExecWriter) loadCurrent(ctx context.Context, entryPath, wireRevision string) (Manifest, string, []FieldValue, error) {
	m, path, _, err := w.loadManifest(ctx, entryPath, wireRevision)
	if err != nil {
		return Manifest{}, "", nil, err
	}
	values := make([]FieldValue, len(m.Fields))
	for i, f := range m.Fields {
		value, err := w.store().show(ctx, fieldValuePath(m.BundleID, m.Revision, f.ID))
		if err != nil {
			return Manifest{}, "", nil, fmt.Errorf("gopass: read field value: %w", err)
		}
		if err := ValidFieldValue(value, f.Multiline); err != nil {
			return Manifest{}, "", nil, err
		}
		values[i] = FieldValue{Kind: f.Kind, Name: f.Name, Visibility: f.Visibility, Multiline: f.Multiline, Value: string(value)}
	}
	return m, path, values, nil
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

func (w ExecWriter) commit(ctx context.Context, manifestPath, expectedRevision string, base Manifest, values []FieldValue, oldPaths []string) (FieldSet, error) {
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

	written := make([]string, 0, len(values))
	rollbackError := func(cause error) error {
		if failed := w.cleanup(ctx, written); failed > 0 {
			return fmt.Errorf("%w; cleanup of %d new entries failed", cause, failed)
		}
		return cause
	}
	for i, value := range values {
		path := fieldValuePath(next.BundleID, next.Revision, next.Fields[i].ID)
		if err := w.store().write(ctx, path, []byte(value.Value)); err != nil {
			return FieldSet{}, rollbackError(err)
		}
		written = append(written, path)
		got, err := w.store().show(ctx, path)
		if err != nil || string(got) != value.Value {
			return FieldSet{}, rollbackError(fmt.Errorf("gopass: value verification failed for field %s", next.Fields[i].ID))
		}
	}
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
	if err != nil || string(confirmed) != string(raw) {
		return FieldSet{}, fmt.Errorf("gopass: manifest verification failed")
	}
	set := fieldSetFrom(next, values, wireRevisionOf(raw))
	if failed := w.cleanup(ctx, oldPaths); failed > 0 {
		return set, &CleanupError{Failed: failed}
	}
	return set, nil
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
