# Changelog

All notable changes will be documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and releases are
intended to follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Public project documentation and GitHub contribution workflows.
- `gc` subcommand for unreferenced bundle data, entry rename and clone, field
  picker in forms, quit confirmation, generator presets with entropy estimate,
  TOTP codes, password age, explicit duplicate-password check, and confirmed
  deletion of linked Bitwarden items.

### Changed

- The main entry of a field bundle now starts with the password.
- The list preview no longer decrypts secret fields.
- Old revisions are removed with one `gopass rm -r` instead of one command per value.

[Unreleased]: https://github.com/andrey-losikhin/zer0-gopass-tui/commits/main
