package gopass

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// mustID генерирует canonical opaque ID для использования в тестовых манифестах.
func mustID(t *testing.T) string {
	t.Helper()
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID() error = %v", err)
	}
	return id
}

func fieldJSON(id, kind, name, visibility string, multiline bool) string {
	return fmt.Sprintf(`{"id":%q,"kind":%q,"name":%q,"visibility":%q,"multiline":%t}`, id, kind, name, visibility, multiline)
}

func manifestJSON(format, bundleID, revision string, fields ...string) []byte {
	return []byte(fmt.Sprintf(`{"format":%q,"bundle_id":%q,"revision":%q,"fields":[%s]}`, format, bundleID, revision, strings.Join(fields, ",")))
}

func TestParseManifest_Valid(t *testing.T) {
	bundleID := mustID(t)
	revision := mustID(t)
	pwID := mustID(t)
	userID := mustID(t)
	customID := mustID(t)

	raw := manifestJSON(manifestFormat, bundleID, revision,
		fieldJSON(pwID, "password", "Password", "secret", false),
		fieldJSON(userID, "username", "Username", "public", false),
		fieldJSON(customID, "custom", "My Custom", "secret", true),
	)

	got, err := ParseManifest(raw)
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}

	want := Manifest{
		Format:   manifestFormat,
		BundleID: bundleID,
		Revision: revision,
		Fields: []Field{
			{ID: pwID, Kind: "password", Name: "Password", Visibility: VisibilitySecret, Multiline: false},
			{ID: userID, Kind: "username", Name: "Username", Visibility: VisibilityPublic, Multiline: false},
			{ID: customID, Kind: "custom", Name: "My Custom", Visibility: VisibilitySecret, Multiline: true},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseManifest() = %+v, want %+v", got, want)
	}
}

func TestParseManifest_Rejects(t *testing.T) {
	bundleID := mustID(t)
	revision := mustID(t)
	id1 := mustID(t)
	id2 := mustID(t)

	validField := fieldJSON(id1, "custom", "Name One", "public", false)

	var manyFields []string
	for i := 0; i < 65; i++ {
		fid := mustID(t)
		manyFields = append(manyFields, fieldJSON(fid, "custom", fmt.Sprintf("Field %d", i), "public", false))
	}

	tests := []struct {
		name string
		raw  []byte
	}{
		{
			name: "exceeds 64 KiB size limit",
			raw:  bytes.Repeat([]byte("a"), 65*1024),
		},
		{
			name: "invalid UTF-8",
			raw:  []byte{0x7b, 0xff, 0xfe, 0x7d},
		},
		{
			name: "unknown top-level key",
			raw:  []byte(fmt.Sprintf(`{"format":%q,"bundle_id":%q,"revision":%q,"fields":[%s],"extra":"x"}`, manifestFormat, bundleID, revision, validField)),
		},
		{
			name: "trailing top-level JSON value",
			raw:  append(manifestJSON(manifestFormat, bundleID, revision, validField), []byte("\n{}")...),
		},
		{
			name: "unknown key inside field object",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fmt.Sprintf(`{"id":%q,"kind":"custom","name":"N","visibility":"public","multiline":false,"extra":"x"}`, id1)),
		},
		{
			name: "duplicate top-level JSON key",
			raw:  []byte(fmt.Sprintf(`{"format":%q,"format":%q,"bundle_id":%q,"revision":%q,"fields":[%s]}`, manifestFormat, manifestFormat, bundleID, revision, validField)),
		},
		{
			name: "unsupported format",
			raw:  manifestJSON("wrong-format", bundleID, revision, validField),
		},
		{
			name: "invalid bundle_id",
			raw:  manifestJSON(manifestFormat, "not-canonical-id", revision, validField),
		},
		{
			name: "invalid revision",
			raw:  manifestJSON(manifestFormat, bundleID, "not-canonical-id", validField),
		},
		{
			name: "zero fields",
			raw:  manifestJSON(manifestFormat, bundleID, revision),
		},
		{
			name: "65 fields exceeds max",
			raw:  manifestJSON(manifestFormat, bundleID, revision, manyFields...),
		},
		{
			name: "invalid field id (not canonical)",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fieldJSON("short-id", "custom", "N", "public", false)),
		},
		{
			name: "duplicate field id",
			raw: manifestJSON(manifestFormat, bundleID, revision,
				fieldJSON(id1, "custom", "Name A", "public", false),
				fieldJSON(id1, "custom", "Name B", "public", false),
			),
		},
		{
			name: "duplicate field name",
			raw: manifestJSON(manifestFormat, bundleID, revision,
				fieldJSON(id1, "custom", "Same Name", "public", false),
				fieldJSON(id2, "custom", "Same Name", "public", false),
			),
		},
		{
			name: "unknown field kind",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fieldJSON(id1, "bogus_kind", "N", "public", false)),
		},
		{
			name: "standard kind visibility mismatch",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fieldJSON(id1, "password", "Password", "public", false)),
		},
		{
			name: "custom kind empty visibility",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fieldJSON(id1, "custom", "N", "", false)),
		},
		{
			name: "empty field name",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fieldJSON(id1, "custom", "", "public", false)),
		},
		{
			name: "field name with control character",
			raw:  manifestJSON(manifestFormat, bundleID, revision, fieldJSON(id1, "custom", "na\x01me", "public", false)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseManifest(tt.raw)
			if err == nil {
				t.Fatalf("ParseManifest() error = nil, want error")
			}
			if !reflect.DeepEqual(got, Manifest{}) {
				t.Fatalf("ParseManifest() on error = %+v, want zero Manifest", got)
			}
		})
	}
}
