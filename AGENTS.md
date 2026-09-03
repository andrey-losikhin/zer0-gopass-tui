# Repository guidance

## Project

Go terminal UI for structured credentials stored through the external `gopass`
CLI. Optional Bitwarden synchronization uses the configured Vault Management API.

## Commands

```sh
go test ./...
go vet ./...
go build ./cmd/zer0-gopass-tui
```

## Constraints

- Preserve Go 1.26 compatibility.
- Never expose secret values in logs, errors, argv, fixtures, or snapshots.
- Invoke external commands with fixed argv and no shell.
- Keep destructive operations explicitly confirmed and concurrency checks intact.
- Tests must use synthetic credentials and disposable stores.
- Update public documentation when behavior, storage, trust boundaries, or
  release behavior changes.
