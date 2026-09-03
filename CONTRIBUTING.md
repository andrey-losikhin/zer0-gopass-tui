# Contributing

Thank you for helping improve zer0-gopass-tui.

## Before opening an issue

- Use the support channels in [SUPPORT.md](SUPPORT.md) for usage questions.
- Search existing issues before filing a bug or feature request.
- Report vulnerabilities privately as described in [SECURITY.md](SECURITY.md).
- Never include passwords, tokens, vault contents, entry names, or sensitive logs.

## Development setup

Install Go 1.26 or newer, clone the repository, and run:

```sh
go mod download
go test ./...
go vet ./...
go build ./cmd/zer0-gopass-tui
```

Most tests use fakes and do not require a real password store. Manual testing
requires `gopass`; use a disposable store and never test destructive behavior
against important credentials.

## Pull requests

1. Keep each pull request focused and explain the user-visible behavior.
2. Add or update tests for behavior changes.
3. Run the development checks above.
4. Update documentation and `CHANGELOG.md` under `Unreleased` when relevant.
5. Use clear commit messages and complete the pull request template.

By submitting a contribution, you agree that it is licensed under the Apache
License 2.0 and that you have the right to contribute it.
