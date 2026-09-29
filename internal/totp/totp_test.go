package totp

import (
	"encoding/base32"
	"testing"
	"time"
)

// Векторы из RFC 6238, Appendix B (8 цифр, период 30 с).
func TestRFC6238Vectors(t *testing.T) {
	seeds := map[string][]byte{
		"SHA1":   []byte("12345678901234567890"),
		"SHA256": []byte("12345678901234567890123456789012"),
		"SHA512": []byte("1234567890123456789012345678901234567890123456789012345678901234"),
	}
	vectors := []struct {
		unix int64
		want map[string]string
	}{
		{59, map[string]string{"SHA1": "94287082", "SHA256": "46119246", "SHA512": "90693936"}},
		{1111111109, map[string]string{"SHA1": "07081804", "SHA256": "68084774", "SHA512": "25091201"}},
		{1111111111, map[string]string{"SHA1": "14050471", "SHA256": "67062674", "SHA512": "99943326"}},
		{1234567890, map[string]string{"SHA1": "89005924", "SHA256": "91819424", "SHA512": "93441116"}},
		{2000000000, map[string]string{"SHA1": "69279037", "SHA256": "90698825", "SHA512": "38618901"}},
		{20000000000, map[string]string{"SHA1": "65353130", "SHA256": "77737706", "SHA512": "47863826"}},
	}
	for _, vector := range vectors {
		for algorithm, want := range vector.want {
			p := Params{Secret: seeds[algorithm], Digits: 8, Period: 30, Algorithm: algorithm}
			got, _, err := Code(p, time.Unix(vector.unix, 0))
			if err != nil || got != want {
				t.Errorf("%s at %d = %q, %v; want %q", algorithm, vector.unix, got, err, want)
			}
		}
	}
}

func TestParseBase32AndURI(t *testing.T) {
	encoded := base32.StdEncoding.EncodeToString([]byte("12345678901234567890"))
	p, err := Parse(" " + encoded[:8] + " " + encoded[8:] + " ")
	if err != nil || string(p.Secret) != "12345678901234567890" || p.Digits != 6 || p.Period != 30 {
		t.Fatalf("Parse(base32) = %#v, %v", p, err)
	}
	p, err = Parse("otpauth://totp/Example:alice?secret=" + encoded + "&digits=8&period=60&algorithm=SHA256")
	if err != nil || p.Digits != 8 || p.Period != 60 || p.Algorithm != "SHA256" {
		t.Fatalf("Parse(uri) = %#v, %v", p, err)
	}
	code, remaining, err := Code(p, time.Unix(90, 0))
	if err != nil || len(code) != 8 || remaining != 30 {
		t.Fatalf("Code() = %q, %d, %v", code, remaining, err)
	}
	for _, bad := range []string{"", "not base32 !", "otpauth://hotp/x?secret=" + encoded, "otpauth://totp/x?secret=" + encoded + "&algorithm=MD5", "otpauth://totp/x?secret=" + encoded + "&digits=3"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}
