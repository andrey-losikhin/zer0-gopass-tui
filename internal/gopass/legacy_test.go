package gopass

import "testing"

func TestParseLegacyMapsStandardAndCustomFields(t *testing.T) {
	raw := []byte("synthetic-pass\nlogin: alice\nWebsite: https://example.test\nPIN code: 1234\notpauth://totp/x?secret=JBSWY3DPEHPK3PXP\nfree text line\nuser: duplicate\n" + compatibilityMarker)
	fields := ParseLegacy(raw)
	byKind := map[FieldKind]FieldValue{}
	var custom []FieldValue
	for _, field := range fields {
		if field.Kind == fieldKindCustom {
			custom = append(custom, field)
			continue
		}
		byKind[field.Kind] = field
	}
	if byKind["password"].Value != "synthetic-pass" || byKind["username"].Value != "alice" || byKind["url"].Value != "https://example.test" {
		t.Fatalf("standard fields = %#v", byKind)
	}
	if byKind["totp_secret"].Value != "otpauth://totp/x?secret=JBSWY3DPEHPK3PXP" {
		t.Fatalf("totp = %q", byKind["totp_secret"].Value)
	}
	if len(custom) != 1 || custom[0].Name != "PIN code" || custom[0].Visibility != VisibilitySecret {
		t.Fatalf("custom = %#v", custom)
	}
	if byKind["notes"].Value != "free text line\nuser: duplicate" {
		t.Fatalf("notes = %q", byKind["notes"].Value)
	}
	for _, field := range fields {
		if err := ValidFieldValue([]byte(field.Value), field.Multiline); err != nil {
			t.Fatalf("field %s invalid: %v", field.Name, err)
		}
	}
}

func TestParseLegacyEmptyFirstLineHasNoPassword(t *testing.T) {
	fields := ParseLegacy([]byte("\nusername: bob"))
	if len(fields) != 1 || fields[0].Kind != "username" {
		t.Fatalf("fields = %#v", fields)
	}
}
