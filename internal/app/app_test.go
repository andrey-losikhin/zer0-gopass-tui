package app

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andrey-losikhin/zer0-gopass-tui/internal/gopass"
)

func TestRunQuitsOnQ(t *testing.T) {
	var stdout bytes.Buffer
	r := &fakeReader{sets: map[string]gopass.FieldSet{}}
	w := &fakeWriter{}
	if err := Run(strings.NewReader("q"), &stdout, fakeLister{}, r, w); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
