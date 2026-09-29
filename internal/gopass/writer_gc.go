package gopass

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const reservedRootDir = ".zer0-waypass/v1"

// storeFileExtensions — расширения зашифрованных файлов backend-ов gopass
// (gpgcli, age, plain). Файлы с другими расширениями gc не трогает.
var storeFileExtensions = []string{".gpg", ".age", ".txt"}

// GCReport описывает найденный мусор в зарезервированном namespace. Все пути —
// opaque gopass-пути без секретов, кроме DetachedManifests (путь manifest, по
// которому восстанавливается путь записи; такие manifest только отображаются).
type GCReport struct {
	Manifests         int
	OrphanBundles     []string
	OrphanRevisions   []string
	OrphanValues      []string
	EmptyDirs         []string
	DetachedManifests []string
}

// Empty сообщает, что удалять нечего.
func (r GCReport) Empty() bool {
	return len(r.OrphanBundles)+len(r.OrphanRevisions)+len(r.OrphanValues)+len(r.EmptyDirs) == 0
}

// GCResult — итог применения отчёта.
type GCResult struct {
	Removed int
	Failed  int
}

type referencedBundle struct {
	revision string
	fields   map[string]bool
}

// GarbageCollect находит value-файлы и каталоги revision/bundle, на которые не
// ссылается ни один manifest, а также пустые каталоги, и удаляет их, только если
// confirm вернул true. Любой нечитаемый или невалидный manifest прерывает gc:
// без него нельзя доказать, что values не используются. Manifest без main entry
// не удаляется и не делает свои values сиротами — он лишь попадает в отчёт.
func (w ExecWriter) GarbageCollect(ctx context.Context, confirm func(GCReport) bool) (GCReport, GCResult, error) {
	unlock, err := w.lockExclusiveGC(ctx)
	if err != nil {
		return GCReport{}, GCResult{}, err
	}
	defer unlock()
	root, err := w.store().root(ctx)
	if err != nil {
		return GCReport{}, GCResult{}, err
	}
	report, err := w.scanGarbage(ctx, root)
	if err != nil || report.Empty() || !confirm(report) {
		return report, GCResult{}, err
	}
	var result GCResult
	count := func(err error) {
		if err != nil {
			result.Failed++
		} else {
			result.Removed++
		}
	}
	for _, dir := range append(slices.Clone(report.OrphanBundles), report.OrphanRevisions...) {
		count(w.store().removeTree(ctx, dir))
	}
	for _, path := range report.OrphanValues {
		count(w.store().remove(ctx, path))
	}
	removed, failed := removeEmptyDirs(filepath.Join(root, filepath.FromSlash(reservedRootDir)))
	result.Removed += removed
	result.Failed += failed
	return report, result, nil
}

func (w ExecWriter) scanGarbage(ctx context.Context, root string) (GCReport, error) {
	base := filepath.Join(root, filepath.FromSlash(reservedRootDir))
	info, err := os.Lstat(base)
	if errors.Is(err, fs.ErrNotExist) {
		return GCReport{}, nil
	}
	if err != nil || !info.IsDir() {
		return GCReport{}, fmt.Errorf("gopass: reserved namespace is not a directory")
	}
	var report GCReport
	referenced, err := w.referencedBundles(ctx, base, &report)
	if err != nil {
		return GCReport{}, err
	}
	bundles, err := os.ReadDir(base)
	if err != nil {
		return GCReport{}, fmt.Errorf("gopass: read reserved namespace: %w", err)
	}
	// Пустые каталоги gopass не отслеживает: они удаляются напрямую, а не через rm -r.
	report.EmptyDirs = emptyDirs(base)
	empty := make(map[string]bool, len(report.EmptyDirs))
	for _, dir := range report.EmptyDirs {
		empty[dir] = true
	}
	for _, bundle := range bundles {
		if !bundle.IsDir() || !ValidCanonicalID(bundle.Name()) || empty[filepath.Join(base, bundle.Name())] {
			continue
		}
		bundlePath := reservedRootDir + "/" + bundle.Name()
		ref, ok := referenced[bundle.Name()]
		if !ok {
			report.OrphanBundles = append(report.OrphanBundles, bundlePath)
			continue
		}
		revisions, err := os.ReadDir(filepath.Join(base, bundle.Name()))
		if err != nil {
			return GCReport{}, fmt.Errorf("gopass: read bundle directory: %w", err)
		}
		for _, revision := range revisions {
			if !revision.IsDir() || !ValidCanonicalID(revision.Name()) || empty[filepath.Join(base, bundle.Name(), revision.Name())] {
				continue
			}
			revisionPath := bundlePath + "/" + revision.Name()
			if revision.Name() != ref.revision {
				report.OrphanRevisions = append(report.OrphanRevisions, revisionPath)
				continue
			}
			values, err := os.ReadDir(filepath.Join(base, bundle.Name(), revision.Name()))
			if err != nil {
				return GCReport{}, fmt.Errorf("gopass: read revision directory: %w", err)
			}
			for _, value := range values {
				id, ok := storeEntryName(value)
				if ok && !ref.fields[id] {
					report.OrphanValues = append(report.OrphanValues, revisionPath+"/"+id)
				}
			}
		}
	}
	return report, nil
}

func (w ExecWriter) referencedBundles(ctx context.Context, base string, report *GCReport) (map[string]referencedBundle, error) {
	referenced := make(map[string]referencedBundle)
	manifests, err := os.ReadDir(filepath.Join(base, "manifests"))
	if errors.Is(err, fs.ErrNotExist) {
		return referenced, nil
	}
	if err != nil {
		return nil, fmt.Errorf("gopass: read manifests directory: %w", err)
	}
	for _, file := range manifests {
		id, ok := storeFileBase(file)
		if !ok {
			continue
		}
		// Любой нераспознанный manifest-файл прерывает gc: иначе его values
		// были бы ошибочно признаны сиротами.
		entryPath, err := DecodeEntryID(id)
		if err != nil {
			return nil, fmt.Errorf("gopass: gc aborted: manifest with invalid entry id")
		}
		manifestPath := manifestPathPrefix + id
		raw, err := w.store().show(ctx, manifestPath)
		if err != nil {
			return nil, fmt.Errorf("gopass: gc aborted: manifest %s is unreadable", id)
		}
		m, err := ParseManifest(raw)
		if err != nil {
			return nil, fmt.Errorf("gopass: gc aborted: manifest %s is invalid", id)
		}
		if other, exists := referenced[m.BundleID]; exists && other.revision != m.Revision {
			return nil, fmt.Errorf("gopass: gc aborted: bundle is shared by several manifests")
		}
		fields := make(map[string]bool, len(m.Fields))
		for _, field := range m.Fields {
			fields[field.ID] = true
		}
		referenced[m.BundleID] = referencedBundle{revision: m.Revision, fields: fields}
		report.Manifests++
		mainExists, err := w.store().exists(ctx, entryPath)
		if err != nil {
			return nil, err
		}
		if !mainExists {
			report.DetachedManifests = append(report.DetachedManifests, manifestPath)
		}
	}
	return referenced, nil
}

// storeFileBase возвращает имя обычного файла store без расширения backend-а.
func storeFileBase(entry fs.DirEntry) (string, bool) {
	if !entry.Type().IsRegular() {
		return "", false
	}
	for _, ext := range storeFileExtensions {
		if base, ok := strings.CutSuffix(entry.Name(), ext); ok {
			return base, true
		}
	}
	return "", false
}

// storeEntryName возвращает canonical ID value-файла без расширения.
func storeEntryName(entry fs.DirEntry) (string, bool) {
	id, ok := storeFileBase(entry)
	return id, ok && ValidCanonicalID(id)
}

// emptyDirs перечисляет каталоги внутри base (кроме самого base и manifests),
// не содержащие ни одного файла, от самых глубоких к верхним.
func emptyDirs(base string) []string {
	var dirs []string
	var walk func(dir string) bool
	walk = func(dir string) bool {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return false
		}
		empty := true
		for _, entry := range entries {
			if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
				if !walk(filepath.Join(dir, entry.Name())) {
					empty = false
				}
				continue
			}
			empty = false
		}
		if empty && dir != base && dir != filepath.Join(base, "manifests") {
			dirs = append(dirs, dir)
		}
		return empty
	}
	walk(base)
	return dirs
}

func removeEmptyDirs(base string) (int, int) {
	removed, failed := 0, 0
	for _, dir := range emptyDirs(base) {
		if err := os.Remove(dir); err != nil {
			failed++
		} else {
			removed++
		}
	}
	return removed, failed
}

// expandStoreRoot раскрывает "~" в выводе `gopass config mounts.path`.
func expandStoreRoot(path string) (string, error) {
	if rest, ok := strings.CutPrefix(path, "~"); ok && (rest == "" || strings.HasPrefix(rest, "/")) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("gopass: resolve home directory: %w", err)
		}
		path = filepath.Join(home, rest)
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("gopass: store root is not absolute")
	}
	return filepath.Clean(path), nil
}
