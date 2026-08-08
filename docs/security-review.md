# Security Review: pm-cli

**Version reviewed**: 0.2.6
**Scope**: full codebase (every non-test source file), dependency scan, threat model
around credentials, IMAP/SMTP protocol handling, terminal output, and filesystem writes

## Summary

The security work in 0.2.5 holds up. The loopback guard genuinely constrains the
`InsecureSkipVerify` exposure, `safetext` is applied at effectively every header and
terminal sink, and credentials go to the OS keyring with restrictive permissions on
every file the tool creates. `govulncheck` reports no vulnerabilities.

This review found one issue introduced by 0.2.6 itself and several pre-existing ones.
All were fixed before the 0.2.6 release.

## Findings

| # | Severity | Issue | Status |
|---|----------|-------|--------|
| 1 | High | Bridge password inherited by `mail watch --exec` children | Fixed (#20) |
| 2 | High | `mail download` silently overwrote files in the working directory | Fixed (#21) |
| 3 | Medium | Crafted Subject could forge rows in `mail list` output | Fixed (#21) |
| 4 | Medium | Idempotency guard was racy and failed open | Fixed (#22) |
| 5 | Medium | Bodies declared quoted-printable but sent raw; write errors dropped | Fixed (#23) |
| 6 | Low | Attachment read error discarded | Fixed (#21) |
| 7 | Low | Subject truncation split UTF-8 characters | Fixed (#21) |
| 8 | Low | `--help-json` matched flag values anywhere in argv | Fixed (#24) |
| 9 | Low | Credential source not reported by `config show` / `doctor` | Fixed (#24) |
| 10 | Low | CI workflow declared no token permissions | Fixed (#24) |
| 11 | Low | `golang.org/x/sys` advisory (Windows, unreachable) | Fixed (#24) |

### 1. Bridge password inherited by child processes (High)

`mail watch --exec` built its child environment with `append(os.Environ(), ...)`. Once
0.2.6 added `PM_CLI_BRIDGE_PASSWORD`, that variable was inherited by the user-supplied
command and by anything it shelled out to, handing the mail credential to arbitrary
third-party scripts.

The env-var credential path and the spawn site were each sound alone; the exposure
existed only once both were present, making this a regression introduced by the
unreleased version. Child environments are now built through `config.ScrubSecrets`.

### 2. Attachment download overwrote existing files (High)

`filepath.Base` stops path traversal, but the surviving basename is chosen by the
sender, and `os.WriteFile` opens with `O_TRUNC`. Running `mail download` from a home
directory against an attachment named `../../../.bashrc` replaced the shell profile
with no prompt, giving code execution on next login. Writes now use `O_EXCL` unless
`--force` is passed.

### 3. Forged rows in list output (Medium)

`SanitizeForTerminal` preserves newlines, which is correct for message bodies but not
for a Subject rendered as one cell of a tab-delimited table. A newline inside an
RFC 2047 encoded-word survives envelope decoding, letting a crafted Subject close its
row and emit further, entirely fake rows. `SanitizeSingleLine` now folds newlines and
tabs for single-line fields; body rendering is unchanged.

### 4. Idempotency guard racy and fail-open (Medium)

The key was checked early and recorded only after the send returned, with the send
between them, so two concurrent invocations both saw the key as unused and both sent.
Separately, a JSON parse error returned an empty store, silently disabling the
protection. Reservations are now one marker file per key created with `O_EXCL`, which
is atomic across processes and has no parsed content that can fail open.

### 5. Transfer encoding mismatch (Medium)

Messages declared `Content-Transfer-Encoding: quoted-printable` and wrote the raw body,
so conforming receivers decoded `=` as an escape and non-ASCII went out as undeclared
8-bit content. Several write errors were also discarded, allowing a truncated message
to be reported as sent.

## Accepted risks

- **TLS certificate verification is disabled.** Required by Proton Bridge's
  self-signed certificate. Constrained by the loopback guard, which refuses to connect
  to any non-loopback host.
- **Passwords live in memory as Go strings.** They cannot be reliably zeroed. Mitigated
  by the short-lived nature of CLI invocations.
- **`PM_CLI_BRIDGE_PASSWORD` is readable by same-user processes** and appears in
  `/proc/<pid>/environ`. This is inherent to environment variables; see the credential
  section of `SECURITY.md`. The keyring remains the default and preferred path.
- **Messages and attachments are read fully into memory.** A very large message is
  bounded by available RAM rather than a configured limit. Only `mail batch` input is
  explicitly capped (10 MB).
- **Downloaded attachments are written `0644`**, readable by other users on a shared
  machine.

## Known gaps

- The `--help-json` schema omits several commands, including `mail download`,
  `attachments`, `watch`, `reply`, `forward`, and `thread`. This weakens the documented
  agent-integration contract without being a vulnerability. Tracked separately.
- `mail search` returns a different JSON shape than `mail list`, since only the list
  path was updated with the new envelope fields. Tracked in #19.

## Method

Every non-test source file was read. Claims were verified by execution rather than
inspection alone: the credential leak, the row forgery, the file overwrite, the UTF-8
truncation, and the `--help-json` argument matching were each reproduced against a
running binary or a real in-memory IMAP server before being fixed, and the same
reproductions were re-run afterwards.
