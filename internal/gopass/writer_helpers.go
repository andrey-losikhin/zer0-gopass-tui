package gopass

import (
	"context"
	"errors"
	"fmt"
)

func (w ExecWriter) rollbackCreated(ctx context.Context, manifestPath string) int {
	raw, err := w.store().show(ctx, manifestPath)
	if err != nil {
		return 1
	}
	m, err := ParseManifest(raw)
	if err != nil {
		return 1
	}
	failed := 0
	if err := w.store().remove(ctx, manifestPath); err != nil {
		return 1
	}
	failed += w.cleanup(ctx, oldValuePaths(m))
	return failed
}

func encodedManifestPath(entryPath string) (string, error) {
	id, err := EncodeCanonicalPath(entryPath)
	if err != nil {
		return "", fmt.Errorf("gopass: invalid entry path: %w", err)
	}
	return manifestPathPrefix + id, nil
}

func fieldIndex(fields []Field, id string) int {
	if !ValidCanonicalID(id) {
		return -1
	}
	for i := range fields {
		if fields[i].ID == id {
			return i
		}
	}
	return -1
}

var errManifestExists = errors.New("gopass: manifest already exists")
var errEntryExists = errors.New("gopass: entry already exists")
