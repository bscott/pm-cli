# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- `mail move`, `mail archive`, and batch move/archive operations now use the
  server's atomic IMAP `MOVE` command when available. This removes a data-loss
  path where the previous unqualified `EXPUNGE` could permanently delete
  unrelated messages already marked `\\Deleted`; the library fallback uses
  targeted `UID EXPUNGE` on UIDPLUS servers (#28, #29).

## [0.2.6] - 2026-08-09

Validated against Proton Bridge 3.25.0 before release: the IMAP no-match detection,
server-side `--unread`/`--flagged` filtering, and the archive and move paths were each
exercised against a live Bridge rather than only the in-memory test server. Bridge
advertises UIDPLUS, so the COPYUID check added in this release operates at full
strength.

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
- `mail watch --exec` no longer passes `PM_CLI_BRIDGE_PASSWORD` to the command it
  runs. The child environment was built from `os.Environ()`, so a Bridge password
  supplied via the environment variable added in this release was inherited by the
  user-supplied command and by anything it shelled out to. Child environments are now
  built through `config.ScrubSecrets`.
- `mail download` refuses to overwrite an existing file unless `--force` is given.
  The default output name comes from the sender-controlled MIME filename, so a
  crafted attachment could previously replace a file in the working directory
  (`.bashrc`, for example) with no prompt.
- Single-line display fields (Subject, From, To, attachment filenames) have newlines
  and tabs folded to spaces before rendering. A newline inside an RFC 2047
  encoded-word survives envelope decoding, which let a crafted Subject close its row
  and forge additional fake rows in `mail list` output.
- `mail list` truncates long Subject and From values by rune rather than by byte, so
  a multi-byte character is no longer split into invalid UTF-8.
- A failed read of attachment data is reported instead of discarded. Previously a
  short read was written to disk as though it were the complete attachment.
- Idempotency keys are reserved before a send is attempted rather than recorded after
  it succeeds, closing a window in which two concurrent invocations both saw the key
  as unused and both sent. Reservations are now one marker file per key created with
  `O_EXCL`, which is atomic across processes; a failed send releases the key so it can
  be retried. Previously a corrupt store also silently disabled the protection
  entirely, because a JSON parse error returned an empty store.

- Outgoing message bodies are encoded as quoted-printable, matching the
  `Content-Transfer-Encoding` header the message has always declared. The raw body was
  written verbatim, so conforming receivers decoded any `=` in the text as an escape
  sequence and non-ASCII text went out as undeclared 8-bit content. Affects
  `mail send`, `mail reply`, and `mail forward`, with and without attachments.
- Write errors while building a message are reported instead of discarded. A failure
  partway through an attachment or the multipart trailer previously produced a
  truncated message that was sent as though complete.

- `--help-json` is only recognized as a leading flag. It was matched anywhere in the
  argument list, so `mail send -s "--help-json"` printed the schema and exited 0
  without sending, which a calling script could not distinguish from success.
- `config show` and `config doctor` report which credential is in use. Both said
  "stored in keyring" even when `PM_CLI_BRIDGE_PASSWORD` supplied the password, and
  `doctor` reported "cannot check" when the variable was set without a configured
  email, despite the credential working.
- Bumped `golang.org/x/sys` to v0.44.0 for GO-2026-5024. The affected symbol is
  Windows-only and was not reachable from this code.
- The CI workflow declares `permissions: contents: read` rather than relying on the
  default token scope.

### Removed

- The `idempotency.json` store is replaced by an `idempotency/` directory. No
  migration is performed: keys expire after 24 hours, so the old file simply goes
  unused and can be deleted.

### Changed
- `mail list --unread` now returns up to `--limit` unread messages. Previously the
  limit was applied before filtering, so the command could return far fewer results
  than requested (#9).
