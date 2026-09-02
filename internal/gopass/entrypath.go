package gopass

import (
	"encoding/base64"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// entryIDVersion — версия формата EntryID (см. FIELD-CONTRACT.md v1).
// Первый байт payload'а перед кодированием в base64url.
const entryIDVersion byte = 0x01

// maxCanonicalPathBytes — максимальная длина canonical path в UTF-8 байтах.
const maxCanonicalPathBytes = 4096

// shellMetaChars — ASCII-символы, недопустимые в сегментах canonical path,
// так как могут быть интерпретированы как метасимволы shell/CLI-флаги.
// "/" в этот набор не входит: он является разделителем сегментов пути.
// Пробел намеренно не входит: FIELD-CONTRACT.md v1 явно разрешает пробелы
// в сегментах пути ("Spaces, Unicode names, nested paths, and dotfiles
// remain valid").
const shellMetaChars = ";&|$`><*?[](){}!#~'\""

// EncodeCanonicalPath кодирует пользовательский canonical path записи
// gopass (например "github/account") в opaque reversible EntryID:
// 1 байт версии формата (entryIDVersion) + UTF-8 байты пути, всё в
// unpadded base64url. Путь предварительно валидируется через
// validateCanonicalPath; при невалидном пути возвращается redacted
// ошибка без утечки самого значения path.
func EncodeCanonicalPath(canonicalPath string) (string, error) {
	if err := validateCanonicalPath(canonicalPath); err != nil {
		return "", err
	}
	payload := make([]byte, 0, len(canonicalPath)+1)
	payload = append(payload, entryIDVersion)
	payload = append(payload, canonicalPath...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

// DecodeEntryID декодирует EntryID обратно в canonical path записи gopass.
// Выполняет обратную операцию EncodeCanonicalPath: base64url-decode,
// проверку байта версии формата и повторную валидацию декодированного
// пути через validateCanonicalPath (защита от обхода валидации через
// подложные/мусорные значения entryID). Никакая Unicode-нормализация
// не применяется - сравнение байт-в-байт. Ошибки не содержат entryID
// или декодированный path в тексте.
func DecodeEntryID(entryID string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(entryID)
	if err != nil {
		return "", fmt.Errorf("gopass: invalid entry id")
	}
	if len(decoded) < 1 {
		return "", fmt.Errorf("gopass: invalid entry id")
	}
	if decoded[0] != entryIDVersion {
		return "", fmt.Errorf("gopass: unsupported entry id version")
	}
	path := string(decoded[1:])
	if err := validateCanonicalPath(path); err != nil {
		return "", err
	}
	return path, nil
}

// validateCanonicalPath проверяет, что path является безопасным canonical
// path записи gopass (fail-closed). Отклоняет: пустую строку, невалидный
// UTF-8, путь длиннее maxCanonicalPathBytes, абсолютный путь и пустые
// сегменты (включая "//", ведущий/замыкающий "/"), сегменты "." и ".."
// (traversal), сегменты, начинающиеся с "-" (могли бы быть приняты за
// CLI-флаг), backslash, любые Unicode control code points, bidi control
// characters (изменяющие визуальный порядок отображения, см. isBidiControl)
// и ASCII shell-метасимволы (shellMetaChars). "/" как разделитель сегментов и
// обычные буквы/цифры/точка внутри имени сегмента (например
// "example.com") остаются валидными; пробелы и Unicode-имена в сегментах
// тоже валидны, пока не входят в число control/shell-метасимволов.
func validateCanonicalPath(path string) error {
	if path == "" {
		return fmt.Errorf("gopass: invalid canonical path")
	}
	if !utf8.ValidString(path) {
		return fmt.Errorf("gopass: invalid canonical path")
	}
	if len(path) > maxCanonicalPathBytes {
		return fmt.Errorf("gopass: invalid canonical path")
	}
	if strings.Contains(path, "\\") {
		return fmt.Errorf("gopass: invalid canonical path")
	}
	for _, r := range path {
		if unicode.IsControl(r) || isBidiControl(r) {
			return fmt.Errorf("gopass: invalid canonical path")
		}
		if r != '/' && strings.ContainsRune(shellMetaChars, r) {
			return fmt.Errorf("gopass: invalid canonical path")
		}
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == "" {
			// покрывает абсолютный путь, ведущий/замыкающий "/" и "//"
			return fmt.Errorf("gopass: invalid canonical path")
		}
		if seg == "." || seg == ".." {
			return fmt.Errorf("gopass: invalid canonical path")
		}
		if strings.HasPrefix(seg, "-") {
			return fmt.Errorf("gopass: invalid canonical path")
		}
	}
	return nil
}
