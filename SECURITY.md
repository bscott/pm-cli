# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in pm-cli, please report it responsibly:

1. **Do not** open a public GitHub issue for security vulnerabilities
2. Email the maintainer directly with details of the vulnerability
3. Include steps to reproduce the issue
4. Allow reasonable time for a fix before public disclosure

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Security Measures

### Credential Storage

- **Passwords are never stored in plain text files**
- Bridge passwords are stored in the system keyring using `go-keyring`:
  - Linux: libsecret (GNOME Keyring / KDE Wallet)
  - macOS: Keychain
  - Windows: Windows Credential Manager
- Configuration files (`~/.config/pm-cli/config.yaml`) store only non-sensitive settings
- Config file permissions are set to `0600` (owner read/write only)
- Config directory permissions are set to `0700`

### Network Security

- All connections to Proton Bridge use TLS encryption
- IMAP uses STARTTLS on port 1143
- SMTP uses implicit TLS on port 1025
- Certificate validation is disabled (`InsecureSkipVerify: true`) because Proton Bridge uses self-signed certificates

### Input Handling

- Message IDs are validated as numeric values before use
- File paths for attachments are validated using Kong's `existingfile` type
- Search queries are passed to the IMAP server without shell interpretation

### Data Handling

- Raw message bodies (including `RawBody` and attachment `Data`) are excluded from JSON output by default
- Passwords are read using terminal password input (no echo)
- Error messages do not expose sensitive credential information

## Known Limitations

### TLS Certificate Validation

The tool disables TLS certificate verification (`InsecureSkipVerify: true`) for connections to Proton Bridge. This is necessary because Proton Bridge uses self-signed certificates. This is acceptable because:

1. Proton Bridge runs locally (127.0.0.1)
2. The connection never leaves the local machine
3. Users configure only localhost addresses by default

**Recommendation**: Only use this tool with Proton Bridge running on localhost. Do not configure remote IMAP/SMTP servers.

### Attachment Downloads

- Downloaded attachments are written with permissions `0644`
- The output path for attachments is user-controlled via the `--out` flag
- Path traversal is possible if the user specifies a path outside the current directory (this is intentional to allow saving anywhere)

### Error Message Information Disclosure

Some error messages from the IMAP/SMTP libraries may contain server responses. These do not typically include sensitive data but may reveal server software versions.

## Dependency Security

The project uses the following external dependencies:

| Package | Purpose | Notes |
|---------|---------|-------|
| `github.com/alecthomas/kong` | CLI parsing | Well-maintained |
| `github.com/emersion/go-imap/v2` | IMAP client | Standard Go IMAP library |
| `github.com/emersion/go-message` | Email parsing | Standard email library |
| `github.com/zalando/go-keyring` | System keyring | Uses OS-provided secure storage |
| `golang.org/x/term` | Terminal input | Go extended library |
| `gopkg.in/yaml.v3` | Config parsing | Standard YAML library |

## Best Practices for Users

1. **Keep Proton Bridge updated** - Security fixes are regularly released
2. **Run on localhost only** - Do not expose Bridge ports to the network
3. **Secure your keyring** - Use a strong password for your keyring/wallet
4. **Review config permissions** - Ensure `~/.config/pm-cli/` is not world-readable
5. **Be cautious with attachments** - Downloaded files may contain malware

## Security Audit

This project underwent a code review in February 2026. See `docs/security-review.md` for detailed findings.
