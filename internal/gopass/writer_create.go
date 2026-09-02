package gopass

import (
	"context"
	"fmt"
)

// CreateBundle создаёт bundle, только если manifest и compatibility entry отсутствуют.
func (w ExecWriter) CreateBundle(ctx context.Context, entryPath string, fields []FieldValue) (FieldSet, error) {
	return w.createBundle(ctx, entryPath, fields, false)
}

// MigrateBundle явно создаёт bundle для существующей legacy-записи, сохраняя
// её compatibility entry без изменения.
func (w ExecWriter) MigrateBundle(ctx context.Context, entryPath string, fields []FieldValue) (FieldSet, error) {
	return w.createBundle(ctx, entryPath, fields, true)
}

func (w ExecWriter) createBundle(ctx context.Context, entryPath string, fields []FieldValue, migrate bool) (FieldSet, error) {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	defer unlock()
	manifestPath, err := encodedManifestPath(entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	exists, err := w.store().exists(ctx, manifestPath)
	if err != nil {
		return FieldSet{}, err
	}
	if exists {
		return FieldSet{}, errManifestExists
	}
	compatibilityExists, err := w.store().exists(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	if migrate != compatibilityExists {
		if migrate {
			return FieldSet{}, fmt.Errorf("gopass: legacy entry no longer exists")
		}
		return FieldSet{}, errEntryExists
	}
	bundleID, err := GenerateID()
	if err != nil {
		return FieldSet{}, err
	}
	set, err := w.commit(ctx, manifestPath, "", Manifest{Format: manifestFormat, BundleID: bundleID}, fields, nil)
	if err != nil {
		return set, err
	}
	if migrate {
		compatibilityExists, err = w.store().exists(ctx, entryPath)
		if err != nil || !compatibilityExists {
			failed := w.rollbackCreated(ctx, manifestPath)
			if err != nil {
				return FieldSet{}, fmt.Errorf("gopass: recheck legacy entry: %w", err)
			}
			if failed > 0 {
				return FieldSet{}, fmt.Errorf("gopass: legacy entry disappeared; cleanup of %d bundle entries failed", failed)
			}
			return FieldSet{}, fmt.Errorf("gopass: legacy entry disappeared")
		}
		return set, nil
	}
	compatibilityExists, err = w.store().exists(ctx, entryPath)
	if err != nil || compatibilityExists {
		failed := w.rollbackCreated(ctx, manifestPath)
		if err != nil {
			return FieldSet{}, fmt.Errorf("gopass: recheck compatibility entry: %w", err)
		}
		if failed > 0 {
			return FieldSet{}, fmt.Errorf("%w; cleanup of %d bundle entries failed", errEntryExists, failed)
		}
		return FieldSet{}, errEntryExists
	}
	if err := w.store().write(ctx, entryPath, []byte("zer0-waypass field bundle")); err != nil {
		if failed := w.rollbackCreated(ctx, manifestPath); failed > 0 {
			return FieldSet{}, fmt.Errorf("gopass: create compatibility entry failed; cleanup of %d bundle entries failed: %w", failed, err)
		}
		return FieldSet{}, fmt.Errorf("gopass: create compatibility entry: %w", err)
	}
	return set, nil
}
