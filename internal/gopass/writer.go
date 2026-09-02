package gopass

import (
	"context"
	"fmt"
)

// FieldValue задаёт descriptor и значение поля для создания или изменения.
type FieldValue struct {
	Kind       FieldKind
	Name       string
	Visibility Visibility
	Multiline  bool
	Value      string
}

// CleanupError означает, что новый manifest уже установлен, но часть старых
// value entries удалить не удалось. Обновление успешно и откатывать его нельзя.
type CleanupError struct{ Failed int }

func (e *CleanupError) Error() string {
	return fmt.Sprintf("gopass: bundle updated, but %d old entries were not cleaned up", e.Failed)
}

// Writer создаёт и изменяет field bundles.
type Writer interface {
	CreateBundle(context.Context, string, []FieldValue) (FieldSet, error)
	MigrateBundle(context.Context, string, []FieldValue) (FieldSet, error)
	AddField(context.Context, string, string, FieldValue) (FieldSet, error)
	UpdateField(context.Context, string, string, string, FieldValue) (FieldSet, error)
	ReplaceBundle(context.Context, string, string, []FieldValue) (FieldSet, error)
	DeleteField(context.Context, string, string, string) (FieldSet, error)
	DeleteEntry(context.Context, string, string) error
	DeleteLegacy(context.Context, string) error
}

// ReplaceBundle атомарно заменяет все поля bundle с optimistic-lock.
func (w ExecWriter) ReplaceBundle(ctx context.Context, entryPath, wireRevision string, fields []FieldValue) (FieldSet, error) {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	defer unlock()
	m, path, _, err := w.loadCurrent(ctx, entryPath, wireRevision)
	if err != nil {
		return FieldSet{}, err
	}
	return w.commit(ctx, path, wireRevision, m, fields, oldValuePaths(m))
}

// DeleteLegacy удаляет обычную запись без field manifest.
func (w ExecWriter) DeleteLegacy(ctx context.Context, entryPath string) error {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return err
	}
	defer unlock()
	manifestPath, err := encodedManifestPath(entryPath)
	if err != nil {
		return err
	}
	exists, err := w.store().exists(ctx, manifestPath)
	if err != nil {
		return err
	}
	if exists {
		return ErrStaleRevision
	}
	return w.store().remove(ctx, entryPath)
}

// StandardField возвращает descriptor стандартного kind с контрактными
// name, visibility и multiline. Для custom и неизвестного kind ok == false.
func StandardField(kind FieldKind) (field FieldValue, ok bool) {
	spec, ok := standardFields[kind]
	if !ok {
		return FieldValue{}, false
	}
	return FieldValue{Kind: kind, Name: spec.Name, Visibility: spec.Visibility, Multiline: spec.Multiline}, true
}

// ExecWriter изменяет field bundles через gopass. Каждая мутация переносит все
// оставшиеся значения под новый revision и выдаёт всем полям новые ID. Manifest
// заменяется только после записи и точной проверки всех values. Сбой процесса до
// замены manifest может оставить orphan entries, но старый bundle остаётся рабочим.
type ExecWriter struct {
	backend     writerBackend
	lockEnabled bool
}

func (w ExecWriter) store() writerBackend {
	if w.backend != nil {
		return w.backend
	}
	return execWriterBackend{}
}

// AddField добавляет поле к bundle с optimistic-lock по wireRevision.
func (w ExecWriter) AddField(ctx context.Context, entryPath, wireRevision string, field FieldValue) (FieldSet, error) {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	defer unlock()
	m, path, values, err := w.loadCurrent(ctx, entryPath, wireRevision)
	if err != nil {
		return FieldSet{}, err
	}
	values = append(values, field)
	return w.commit(ctx, path, wireRevision, m, values, oldValuePaths(m))
}

// UpdateField заменяет descriptor и значение существующего поля.
func (w ExecWriter) UpdateField(ctx context.Context, entryPath, wireRevision, fieldID string, field FieldValue) (FieldSet, error) {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	defer unlock()
	m, path, values, err := w.loadCurrent(ctx, entryPath, wireRevision)
	if err != nil {
		return FieldSet{}, err
	}
	index := fieldIndex(m.Fields, fieldID)
	if index < 0 {
		return FieldSet{}, ErrFieldNotFound
	}
	values[index] = field
	return w.commit(ctx, path, wireRevision, m, values, oldValuePaths(m))
}

// DeleteField удаляет поле; удаление последнего поля выполняется DeleteEntry.
func (w ExecWriter) DeleteField(ctx context.Context, entryPath, wireRevision, fieldID string) (FieldSet, error) {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return FieldSet{}, err
	}
	defer unlock()
	m, path, values, err := w.loadCurrent(ctx, entryPath, wireRevision)
	if err != nil {
		return FieldSet{}, err
	}
	index := fieldIndex(m.Fields, fieldID)
	if index < 0 {
		return FieldSet{}, ErrFieldNotFound
	}
	if len(values) == 1 {
		return FieldSet{}, fmt.Errorf("gopass: cannot delete last field; use DeleteEntry")
	}
	values = append(values[:index], values[index+1:]...)
	return w.commit(ctx, path, wireRevision, m, values, oldValuePaths(m))
}

// DeleteEntry удаляет manifest после fresh lock-check, затем все его values.
func (w ExecWriter) DeleteEntry(ctx context.Context, entryPath, wireRevision string) error {
	unlock, err := w.lock(ctx, entryPath)
	if err != nil {
		return err
	}
	defer unlock()
	m, path, _, err := w.loadManifest(ctx, entryPath, wireRevision)
	if err != nil {
		return err
	}
	fresh, err := w.store().show(ctx, path)
	if err != nil || wireRevisionOf(fresh) != wireRevision {
		return ErrStaleRevision
	}
	if err := w.store().remove(ctx, path); err != nil {
		return err
	}
	failed := w.cleanup(ctx, oldValuePaths(m))
	if err := w.store().remove(ctx, entryPath); err != nil {
		failed++
	}
	if failed > 0 {
		return &CleanupError{Failed: failed}
	}
	return nil
}
