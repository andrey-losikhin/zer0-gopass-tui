// Package totp реализует RFC 6238 (TOTP) поверх RFC 4226 (HOTP) на stdlib.
package totp

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Params — параметры генерации кода.
type Params struct {
	Secret    []byte
	Digits    int
	Period    int
	Algorithm string
}

var errInvalid = errors.New("totp: invalid secret")

// Parse принимает base32-секрет или URI otpauth://totp/... Текст ошибок не
// содержит исходного значения.
func Parse(value string) (Params, error) {
	value = strings.TrimSpace(value)
	params := Params{Digits: 6, Period: 30, Algorithm: "SHA1"}
	secret := value
	if strings.HasPrefix(strings.ToLower(value), "otpauth://") {
		uri, err := url.Parse(value)
		if err != nil || !strings.EqualFold(uri.Scheme, "otpauth") {
			return Params{}, errInvalid
		}
		if !strings.EqualFold(uri.Host, "totp") {
			return Params{}, fmt.Errorf("totp: only otpauth://totp is supported")
		}
		query := uri.Query()
		secret = query.Get("secret")
		if raw := query.Get("digits"); raw != "" {
			digits, err := strconv.Atoi(raw)
			if err != nil || digits < 6 || digits > 8 {
				return Params{}, fmt.Errorf("totp: invalid digits")
			}
			params.Digits = digits
		}
		if raw := query.Get("period"); raw != "" {
			period, err := strconv.Atoi(raw)
			if err != nil || period < 1 || period > 300 {
				return Params{}, fmt.Errorf("totp: invalid period")
			}
			params.Period = period
		}
		if raw := query.Get("algorithm"); raw != "" {
			params.Algorithm = strings.ToUpper(raw)
		}
	}
	if hasher(params.Algorithm) == nil {
		return Params{}, fmt.Errorf("totp: unsupported algorithm")
	}
	cleaned := strings.ToUpper(strings.NewReplacer(" ", "", "-", "", "=", "").Replace(secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(cleaned)
	if err != nil || len(key) == 0 {
		return Params{}, errInvalid
	}
	params.Secret = key
	return params, nil
}

// Code возвращает код для момента at и число секунд до смены кода.
func Code(p Params, at time.Time) (string, int, error) {
	h := hasher(p.Algorithm)
	if h == nil || len(p.Secret) == 0 || p.Period < 1 || p.Digits < 6 || p.Digits > 8 {
		return "", 0, errInvalid
	}
	unix := at.Unix()
	if unix < 0 {
		return "", 0, fmt.Errorf("totp: time before epoch")
	}
	period := int64(p.Period)
	counter := uint64(unix / period)
	remaining := int(period - unix%period)
	return hotp(p.Secret, counter, p.Digits, h), remaining, nil
}

func hotp(key []byte, counter uint64, digits int, h func() hash.Hash) string {
	var message [8]byte
	binary.BigEndian.PutUint64(message[:], counter)
	mac := hmac.New(h, key)
	mac.Write(message[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	modulo := uint32(1)
	for range digits {
		modulo *= 10
	}
	return fmt.Sprintf("%0*d", digits, value%modulo)
}

func hasher(algorithm string) func() hash.Hash {
	switch algorithm {
	case "SHA1":
		return sha1.New
	case "SHA256":
		return sha256.New
	case "SHA512":
		return sha512.New
	default:
		return nil
	}
}
