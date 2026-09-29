# Architecture and storage format

## Components

- `cmd/zer0-gopass-tui`: process entry point.
- `internal/app`: Bubble Tea state, forms, rendering, password generation, and
  optional Bitwarden synchronization.
- `internal/gopass`: validated field-bundle model and `gopass` CLI adapter.
- `scripts`: local convenience launchers; they are not privileged installers.

## Trust boundaries

The application trusts local `gopass` and GPG configuration for encryption,
authentication, and durable storage. Entry paths and process metadata are not
secret. Field values, generated passwords, Bitwarden sessions, and API payloads
are sensitive.

Bitwarden synchronization is opt-in per entry. Plain HTTP is accepted only on
loopback; non-loopback endpoints require HTTPS. Synchronization is one-way and
occurs after the `gopass` mutation has committed.

## Field bundles

A logical entry uses an encrypted manifest at:

```text
.zer0-waypass/v1/manifests/<canonical-entry-path-id>
```

The format identifier is `zer0-waypass/fields-v1`. A manifest contains a bundle
ID, revision ID, and 1–64 field descriptors. Each encrypted value is stored at:

```text
.zer0-waypass/v1/<bundle-id>/<revision-id>/<field-id>
```

Manifests are fail-closed: malformed JSON, unknown or duplicate keys, invalid
UTF-8, unsupported fields, and inconsistent metadata are rejected. The exact
manifest bytes are hashed and rechecked before sensitive reads or updates to
detect concurrent changes.

Standard field rules live in `internal/gopass/fields.go`. Custom fields may be
public or secret. “Public” means visible without a separate reveal action in
the TUI; the value is still encrypted by `gopass`.

## Mutation model

Writes are copy-on-write: a new revision is written and verified before the
manifest pointer changes. If the written manifest cannot be confirmed, the
previous manifest bytes are restored on a best-effort basis; a different valid
manifest is treated as a concurrent change and left untouched. The previous
revision directory is removed with a single `gopass rm -r`, so an update costs
one commit per value, one for the manifest and one for cleanup. Cleanup
failures are reported separately after a successful commit.

The compatibility (main) entry keeps normal `gopass` listing and tools working:
its first line is the password field value and the second line is
`zer0-waypass: field bundle`. Saves rewrite only the first line, and only when
it differs from the manifest password; migrated legacy entries keep their other
lines. Deletion removes the manifest first, then the bundle directory and the
main entry; anything left behind by a failure is unreachable and is removed by
`zer0-gopass-tui gc`. Renaming moves the manifest and main entry; value paths
use the opaque bundle ID and stay in place.

## Release model

Pull requests and default-branch pushes run formatting, tests, vet, build, and
CodeQL checks. Versioned binary releases are not automated yet; source installs
use `go install` or a local build.
