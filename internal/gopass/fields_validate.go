package gopass

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"
)

// utf8Valid — тонкая обёртка над unicode/utf8.Valid для единообразия
// с utf8.ValidString, используемой для строковых полей.
func utf8Valid(b []byte) bool {
	return utf8.Valid(b)
}

// checkDuplicateKeys рекурсивно проверяет JSON на дублирующиеся ключи
// в любом объекте на любом уровне вложенности (корневой объект манифеста
// и объекты внутри fields[]). encoding/json.Decoder сам по себе не
// отклоняет дубли ключей (последний перезаписывает предыдущие), поэтому
// проверка выполняется вручную по токенам.
func checkDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("gopass: malformed JSON: %w", err)
	}
	return checkDuplicateKeysValue(dec, tok)
}

// checkDuplicateKeysValue обрабатывает один JSON-токен верхнего значения:
// для объектов проверяет уникальность ключей и рекурсивно спускается
// в значения, для массивов - рекурсивно спускается в элементы.
func checkDuplicateKeysValue(dec *json.Decoder, tok json.Token) error {
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]bool)
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyTok.(string)
			if !ok {
				return fmt.Errorf("gopass: malformed JSON: non-string key")
			}
			if seen[key] {
				return fmt.Errorf("gopass: duplicate key")
			}
			seen[key] = true

			valTok, err := dec.Token()
			if err != nil {
				return err
			}
			if err := checkDuplicateKeysValue(dec, valTok); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil && err != io.EOF { // closing '}'
			return err
		}
	case '[':
		for dec.More() {
			valTok, err := dec.Token()
			if err != nil {
				return err
			}
			if err := checkDuplicateKeysValue(dec, valTok); err != nil {
				return err
			}
		}
		if _, err := dec.Token(); err != nil && err != io.EOF { // closing ']'
			return err
		}
	}
	return nil
}

// validDisplayName проверяет корректность отображаемого имени поля:
// непустое, валидный UTF-8, без control characters (Unicode Cc) и без
// bidi control characters, которые могут визуально подменить текст.
func validDisplayName(name string) bool {
	if name == "" || !utf8.ValidString(name) {
		return false
	}
	for _, r := range name {
		if unicode.IsControl(r) || isBidiControl(r) {
			return false
		}
	}
	return true
}

// isBidiControl сообщает, является ли руна bidi control character
// (LRE/RLE/PDF/LRO/RLO, LRI/RLI/FSI/PDI, LRM/RLM, ALM), способным
// изменить визуальный порядок отображения текста.
func isBidiControl(r rune) bool {
	switch r {
	case '\u200e', '\u200f', // LRM, RLM
		'\u202a', '\u202b', '\u202c', '\u202d', '\u202e', // LRE, RLE, PDF, LRO, RLO
		'\u2066', '\u2067', '\u2068', '\u2069', // LRI, RLI, FSI, PDI
		'\u061c': // ALM
		return true
	default:
		return false
	}
}

// ValidFieldValue проверяет значение поля записи (value policy):
// валидный UTF-8, длина от 1 байта до 1 MiB, только печатные символы
// плюс LF (и TAB, если multiline); CR запрещён всегда, любые прочие
// control characters запрещены везде. Текст ошибки не содержит value.
func ValidFieldValue(value []byte, multiline bool) error {
	if !utf8.Valid(value) {
		return fmt.Errorf("gopass: invalid field value: invalid UTF-8")
	}
	if len(value) < 1 || len(value) > 1024*1024 {
		return fmt.Errorf("gopass: invalid field value: invalid length")
	}
	for _, b := range value {
		switch {
		case b == 0x0A: // LF
			if !multiline {
				return fmt.Errorf("gopass: invalid field value: line break not allowed")
			}
		case b == 0x0D: // CR - запрещён всегда
			return fmt.Errorf("gopass: invalid field value: carriage return not allowed")
		case b == 0x09: // TAB
			if !multiline {
				return fmt.Errorf("gopass: invalid field value: tab not allowed")
			}
		case b < 0x20 || b == 0x7F:
			return fmt.Errorf("gopass: invalid field value: control character not allowed")
		}
	}
	for _, r := range string(value) {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return fmt.Errorf("gopass: invalid field value: control character not allowed")
		}
	}
	return nil
}
