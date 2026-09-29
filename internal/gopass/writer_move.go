package gopass

import (
	"context"
	"fmt"
)

// MoveEntry переименовывает запись from в to. Для field bundle переносятся
// manifest и main entry; каталоги values адресуются opaque bundle_id и
// остаются на месте, поэтому содержимое manifest (и wire revision) не меняется.
// wireRevision == "" означает legacy-запись без manifest: переносится только
// main entry, а появившийся тем временем manifest считается конкурентным изменением.
func (w ExecWriter) MoveEntry(ctx context.Context, from, to, wireRevision string) error {
	if _, err := EncodeCanonicalPath(to); err != nil {
		return fmt.Errorf("gopass: invalid target path: %w", err)
	}
	if from == to {
		return fmt.Errorf("gopass: target path equals source path")
	}
	unlock, err := w.lock(ctx, from, to)
	if err != nil {
		return err
	}
	defer unlock()
	fromManifest, err := encodedManifestPath(from)
	if err != nil {
		return err
	}
	toManifest, err := encodedManifestPath(to)
	if err != nil {
		return err
	}
	for _, path := range []string{to, toManifest} {
		exists, err := w.store().exists(ctx, path)
		if err != nil {
			return err
		}
		if exists {
			return errEntryExists
		}
	}
	if wireRevision == "" {
		exists, err := w.store().exists(ctx, fromManifest)
		if err != nil {
			return err
		}
		if exists {
			return ErrStaleRevision
		}
		return w.store().move(ctx, from, to)
	}
	if _, _, _, err := w.loadManifest(ctx, from, wireRevision); err != nil {
		return err
	}
	if err := w.store().move(ctx, fromManifest, toManifest); err != nil {
		return err
	}
	if err := w.store().move(ctx, from, to); err != nil {
		if backErr := w.store().move(ctx, toManifest, fromManifest); backErr != nil {
			return fmt.Errorf("gopass: move main entry failed and manifest rollback failed: %w", err)
		}
		return fmt.Errorf("gopass: move main entry: %w", err)
	}
	return nil
}
