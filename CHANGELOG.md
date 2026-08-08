# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Targeted for the 0.2.6 release.

### Added
- `mail batch` subcommand: execute multiple label/unlabel/archive/move/flag/delete
  operations from a JSON array over a single IMAP session, with per-op validation and
  an optional `--stop-on-error` flag. Reported by @Juan-de-Costa-Rica (#10).
- Server-side filtering for `mail list` (`--unread` via IMAP `SEARCH UNSEEN`, new
  `--flagged` via `SEARCH FLAGGED`), additional envelope fields (`from_address`, `to`,
  `message_id`, `in_reply_to`), and `--fields`/`--compact` JSON selection.
  Reported by @Juan-de-Costa-Rica (#9), implemented by @kochj23 (#16).
- `PM_CLI_BRIDGE_PASSWORD` environment variable as a fallback for the Bridge password,
  for headless environments with no secret service. Takes precedence over the keyring
  when set and non-empty. Reported by @Juan-de-Costa-Rica (#8), implemented by
  @kochj23 (#14).

### Fixed
- `mail delete`, `mail move`, and label operations no longer report success when the
  target UIDs are not present in the selected mailbox. A STORE that matches nothing is
  a valid no-op per RFC 3501, so the affected-message count is now checked.
  Reported by @Juan-de-Costa-Rica (#11), implemented by @kochj23 (#15).
- The COPY no-match check is gated on the server advertising UIDPLUS, since COPYUID is
  only guaranteed there; without the guard every successful copy on a non-UIDPLUS
  server would have been reported as a failure (#17).

### Changed
- `mail list --unread` now returns up to `--limit` unread messages. Previously the
  limit was applied before filtering, so the command could return far fewer results
  than requested (#9).
