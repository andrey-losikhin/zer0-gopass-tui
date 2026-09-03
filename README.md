# zer0-gopass-tui

[![CI](https://github.com/andrey-losikhin/zer0-gopass-tui/actions/workflows/ci.yml/badge.svg)](https://github.com/andrey-losikhin/zer0-gopass-tui/actions/workflows/ci.yml)
[![CodeQL](https://github.com/andrey-losikhin/zer0-gopass-tui/actions/workflows/codeql.yml/badge.svg)](https://github.com/andrey-losikhin/zer0-gopass-tui/actions/workflows/codeql.yml)
[![License](https://img.shields.io/github/license/andrey-losikhin/zer0-gopass-tui)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/andrey-losikhin/zer0-gopass-tui)](go.mod)

A local, keyboard-first terminal UI for managing structured credentials in
[`gopass`](https://www.gopass.pw/), with optional one-way synchronization to a
local Bitwarden Vault Management API.

> [!IMPORTANT]
> The user interface is currently in Russian. The project is pre-1.0 and the
> encrypted bundle format may change between releases. Back up your password
> store and review changes before upgrading.

## Features

- Search, create, view, edit, and explicitly confirm deletion of entries.
- Store standard and custom fields as encrypted `gopass` entries.
- Reveal secret fields only after an explicit action.
- Generate configurable passwords without placing them in process arguments.
- Migrate legacy single-value entries to structured field bundles.
- Optionally create or update matching Bitwarden Login items.

The application delegates encryption and Git-backed storage to `gopass`. It
does not parse `.gpg` files or manage `gpg-agent`.

## Requirements

- Go 1.26 or newer (when building from source).
- [`gopass`](https://www.gopass.pw/) configured with an accessible password store.
- A terminal with UTF-8 and color support.
- Optional Bitwarden integration: `bw` and `curl`.

## Install

```sh
go install github.com/andrey-losikhin/zer0-gopass-tui/cmd/zer0-gopass-tui@latest
```

Or clone and build a local binary:

```sh
git clone https://github.com/andrey-losikhin/zer0-gopass-tui.git
cd zer0-gopass-tui
go build -o ./bin/zer0-gopass-tui ./cmd/zer0-gopass-tui
./bin/zer0-gopass-tui
```

## Usage

Run `zer0-gopass-tui`, then use:

| Key | Action |
| --- | --- |
| `↑`/`↓` | Select an entry or form row |
| `→`, `l`, `Enter`, `Tab` | Move from the list to the card/form |
| `←`, `h` | Return to the entry list |
| `/` | Search |
| `n` | Create an entry |
| `d` | Delete after confirmation |
| `g` | Open the password generator on a secret field |
| `Ctrl+R` | Reveal or mask a secret during manual input |
| `Ctrl+J` | Add a line break in a multiline field |
| `Ctrl+S` | Save the form |
| `q` | Quit outside text input |

Commands work with English and Russian keyboard layouts.

### Optional Bitwarden synchronization

In a create or full-edit form, press `b` to opt the entry into synchronization.
After a successful `gopass` write, the application creates or updates a
Bitwarden Login item. Disabling synchronization does not delete an existing
Bitwarden item. A Bitwarden failure does not roll back the `gopass` write.

The helper starts `bw serve` on loopback, runs the TUI, and stops the server it
created:

```sh
./scripts/run-with-bitwarden
```

| Variable | Default | Purpose |
| --- | --- | --- |
| `ZER0_GOPASS_BITWARDEN_PORT` | `8088` in the helper | Local `bw serve` port |
| `ZER0_GOPASS_BITWARDEN_URL` | `http://127.0.0.1:8087` in the app | Existing Vault API base URL |

Plain HTTP is accepted only for loopback hosts. Use HTTPS for any remote URL.

### Linux desktop launcher

From a cloned repository, run `./scripts/install-launcher`. It builds the binary
into `${XDG_BIN_HOME:-$HOME/.local/bin}` and installs a per-user desktop entry.

## Security model

- External commands use fixed argument arrays without a shell.
- Secret values are sent to `gopass insert --force` over stdin, not argv.
- Secret fields are not read or displayed until explicitly requested.
- Entry paths are visible metadata; field values remain encrypted by `gopass`.
- Mutating operations use revision checks and deletion requires confirmation.

See [SECURITY.md](SECURITY.md) for private vulnerability reporting and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for storage details and trust boundaries.

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/zer0-gopass-tui
```

Contributions are welcome. Read [CONTRIBUTING.md](CONTRIBUTING.md), the
[Code of Conduct](CODE_OF_CONDUCT.md), and [SUPPORT.md](SUPPORT.md) first.

## License

Licensed under the [Apache License 2.0](LICENSE).
