# Security policy

## Supported versions

This project is pre-1.0. Security fixes are provided for the latest release or,
when no release exists, the default branch only.

## Reporting a vulnerability

Do not open a public issue and do not attach real vault data.

Use GitHub's **Report a vulnerability** form under the repository's Security
tab. Include the affected revision, impact, reproduction steps using synthetic
data, and any suggested mitigation. If private vulnerability reporting is
unavailable, open a minimal public issue asking the maintainer to enable a
private channel; do not disclose technical details there.

You should receive an acknowledgement within 7 days. The maintainer will work
with you on validation, remediation, release timing, and disclosure.

## Scope

The application invokes `gopass` and optionally a Bitwarden Vault Management
API. Upstream vulnerabilities belong with those projects. Secret disclosure,
command execution, unsafe path handling, integrity loss, and insecure API
communication in this repository are in scope.
