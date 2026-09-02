package app

import "zer0-gopass-tui/internal/gopass"

var addableKinds = []gopass.FieldKind{
	"password", "username", "url", "email", "notes", "host", "port",
	"database", "engine", "dsn", "client_id", "jump_host", "api_key",
	"token", "client_secret", "private_key", "passphrase", "sudo_password",
	"totp_secret", "recovery_codes", "custom",
}

func allStandardFields() []gopass.FieldValue {
	fields := make([]gopass.FieldValue, 0, len(addableKinds)-1)
	for _, kind := range addableKinds {
		if kind == "custom" {
			continue
		}
		field, _ := gopass.StandardField(kind)
		fields = append(fields, field)
	}
	return fields
}
