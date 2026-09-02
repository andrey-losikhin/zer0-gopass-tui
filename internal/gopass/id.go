package gopass

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

// idByteLength — количество случайных байт, из которых генерируется
// canonical opaque ID (см. FIELD-CONTRACT.md v1). 16 байт (128 бит)
// в unpadded base64url дают ровно 22 символа.
const idByteLength = 16

// idEncodedLength — длина canonical ID в символах после base64url-кодирования
// без padding: ceil(idByteLength*8/6) = ceil(128/6) = 22.
const idEncodedLength = 22

// GenerateID генерирует canonical opaque ID: 16 случайных байт из
// crypto/rand, закодированных в unpadded base64url (22 символа).
// Используется для bundle_id, revision и field-id.
func GenerateID() (string, error) {
	buf := make([]byte, idByteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("gopass: generate id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ValidCanonicalID проверяет, что id является canonical opaque ID:
// ровно 22 символа алфавита base64url без padding, и результат
// decode->encode совпадает байт-в-байт с исходной строкой. Это
// отклоняет padding ('='), символы стандартного base64 алфавита (+/),
// неверную длину и невалидные (лишние) биты в последнем символе,
// которые RawURLEncoding.DecodeString может декодировать, но которые
// при повторном кодировании дают другую строку.
func ValidCanonicalID(id string) bool {
	if len(id) != idEncodedLength {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(id)
	if err != nil {
		return false
	}
	if len(decoded) != idByteLength {
		return false
	}
	return base64.RawURLEncoding.EncodeToString(decoded) == id
}
