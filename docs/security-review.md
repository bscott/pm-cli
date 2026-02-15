# Security Review: pm-cli

**Review Date**: February 2026
**Reviewer**: Security Code Review
**Version Reviewed**: 0.1.0

## Executive Summary

pm-cli is a Go CLI tool for interacting with ProtonMail via Proton Bridge IMAP/SMTP. The codebase demonstrates generally good security practices, with appropriate use of system keyring for credential storage and proper file permissions. The main security consideration is the intentional disabling of TLS certificate validation, which is acceptable given the localhost-only design.

**Overall Risk Level**: Low (when used as intended with localhost Proton Bridge)

---

## 1. Credential Handling

### 1.1 Password Storage

**File**: `/home/bscott/src/pm-cli/internal/config/config.go`

**Finding**: SECURE

The implementation correctly uses the system keyring for password storage:

```go
func (c *Config) SetPassword(password string) error {
    if c.Bridge.Email == "" {
        return errors.New("email must be set before storing password")
    }
    return keyring.Set(AppName, c.Bridge.Email, password)
}

func (c *Config) GetPassword() (string, error) {
    // ... retrieves from keyring
    password, err := keyring.Get(AppName, c.Bridge.Email)
    // ...
}
```

**Analysis**:
- Passwords are stored in the OS-provided secure credential storage (libsecret on Linux, Keychain on macOS, Credential Manager on Windows)
- No passwords are written to configuration files
- The email address is used as the keyring username, providing per-account isolation

### 1.2 Configuration File Security

**Finding**: SECURE

```go
// Config directory created with restricted permissions
if err := os.MkdirAll(dir, 0700); err != nil {
    return fmt.Errorf("failed to create config directory: %w", err)
}

// Config file created with restricted permissions
if err := os.WriteFile(path, data, 0600); err != nil {
    return fmt.Errorf("failed to write config: %w", err)
}
```

**Analysis**:
- Directory permissions `0700` allow only owner access
- File permissions `0600` prevent group/other read access
- Configuration file contains only non-sensitive data (hosts, ports, email, preferences)

### 1.3 Password Input

**File**: `/home/bscott/src/pm-cli/internal/cli/config.go`

**Finding**: SECURE

```go
passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
```

**Analysis**:
- Uses `golang.org/x/term` for password input
- Characters are not echoed to the terminal
- Password is immediately converted to string and passed to keyring

### 1.4 Password in Memory

**Finding**: ACCEPTABLE

The password is held in memory as a Go string during SMTP client lifetime:

```go
type Client struct {
    config   *config.Config
    password string  // Password stored in struct field
}
```

**Analysis**:
- Go strings are immutable and cannot be securely zeroed
- Password remains in memory until garbage collection
- This is a limitation of the Go language, not the implementation
- Risk is minimal given the short-lived nature of CLI operations

---

## 2. TLS/Cryptographic Security

### 2.1 TLS Certificate Validation

**Files**:
- `/home/bscott/src/pm-cli/internal/imap/client.go`
- `/home/bscott/src/pm-cli/internal/smtp/client.go`

**Finding**: ACCEPTABLE (with caveats)

Both IMAP and SMTP clients disable certificate verification:

```go
// IMAP client
options := &imapclient.Options{
    TLSConfig: &tls.Config{
        InsecureSkipVerify: true,
        ServerName:         c.config.Bridge.IMAPHost,
    },
}

// SMTP client
tlsConfig := &tls.Config{
    InsecureSkipVerify: true,
    ServerName:         c.config.Bridge.SMTPHost,
}
```

**Analysis**:
- Proton Bridge uses self-signed certificates, making verification impossible without certificate pinning
- Default configuration uses `127.0.0.1` (localhost), so traffic never leaves the machine
- TLS still provides encryption even without verification
- **Risk**: If a user configures a remote server, they would be vulnerable to MITM attacks

**Recommendation**:
- Consider adding a warning when IMAP/SMTP hosts are not localhost
- Document that only localhost should be used
- Consider future support for certificate pinning if Proton Bridge provides stable certificates

### 2.2 STARTTLS vs Implicit TLS

**Finding**: CORRECT

- IMAP uses STARTTLS (correct for Proton Bridge port 1143)
- SMTP uses implicit TLS (correct for Proton Bridge port 1025)

---

## 3. Input Validation

### 3.1 Message ID Validation

**File**: `/home/bscott/src/pm-cli/internal/imap/client.go`

**Finding**: SECURE

```go
var seqNum uint32
if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
    return nil, fmt.Errorf("invalid message ID: %s", id)
}
```

**Analysis**:
- Message IDs are parsed as unsigned 32-bit integers
- Invalid input is rejected with an error
- No possibility of IMAP command injection

### 3.2 Mailbox Name Handling

**Finding**: SECURE

Mailbox names are passed directly to the go-imap library, which handles proper IMAP encoding:

```go
if err := c.client.Create(name, nil).Wait(); err != nil {
    return fmt.Errorf("failed to create mailbox %s: %w", name, err)
}
```

**Analysis**:
- The go-imap library handles IMAP string encoding
- No shell execution with mailbox names
- Names are used in IMAP protocol only

### 3.3 Attachment Path Handling

**File**: `/home/bscott/src/pm-cli/internal/cli/cli.go`

**Finding**: SECURE (with intentional user control)

```go
Attach  []string `help:"Attachments" short:"a" type:"existingfile"`
```

For sending, Kong validates that attachment files exist using the `existingfile` type.

For downloading:
```go
outPath := c.Out
if outPath == "" {
    outPath = attachment.Filename
    // ...
}
if err := os.WriteFile(outPath, attachment.Data, 0644); err != nil {
    return fmt.Errorf("failed to write file: %w", err)
}
```

**Analysis**:
- Sending: Attachment paths must be existing files (Kong validation)
- Downloading: Output path is user-controlled, which is intentional
- Download permissions are `0644`, which is standard for user files

### 3.4 Search Query Handling

**Finding**: SECURE

Search queries are parsed by the application and converted to IMAP search criteria:

```go
func parseQueryString(query string) (from, subject, body string) {
    // Parses key:value pairs
    // No shell interpretation
}
```

**Analysis**:
- Queries are parsed into structured SearchOptions
- No shell execution or command injection possible
- Search terms are passed to IMAP protocol handlers

---

## 4. Data Handling

### 4.1 Sensitive Data in JSON Output

**File**: `/home/bscott/src/pm-cli/internal/imap/types.go`

**Finding**: SECURE

```go
type Message struct {
    // ...
    RawBody     []byte       `json:"-"`  // Excluded from JSON
    // ...
}

type Attachment struct {
    // ...
    Data        []byte `json:"-"`  // Excluded from JSON
}
```

**Analysis**:
- Raw message bodies and attachment data are excluded from JSON serialization
- Prevents accidental exposure of binary data in JSON output
- Structured data (headers, metadata) is still available

### 4.2 Error Message Information Disclosure

**Finding**: ACCEPTABLE

Error messages include server responses:

```go
return fmt.Errorf("IMAP login failed: %w", err)
return fmt.Errorf("failed to select mailbox %s: %w", name, err)
```

**Analysis**:
- Error messages may include IMAP/SMTP server responses
- These typically do not contain sensitive data
- May reveal server software version (low risk)
- Useful for debugging connection issues

### 4.3 Email Content Display

**Finding**: SECURE

HTML content is sanitized before display:

```go
func htmlToText(htmlContent string) string {
    // Removes script and style blocks
    reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
    reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
    // ...
    // Strips all HTML tags
    reTags := regexp.MustCompile(`<[^>]+>`)
    text = reTags.ReplaceAllString(text, "")
    // ...
}
```

**Analysis**:
- Script and style blocks are removed
- All HTML tags are stripped
- HTML entities are unescaped for display
- Links are extracted and shown in `[URL]` format

---

## 5. Resource Management

### 5.1 Connection Cleanup

**Finding**: SECURE

All IMAP connections are properly closed:

```go
if err := client.Connect(); err != nil {
    return err
}
defer client.Close()
```

**Analysis**:
- All command handlers use `defer client.Close()`
- Close method handles logout before closing connection
- No connection leaks identified

### 5.2 File Handle Cleanup

**Finding**: SECURE

```go
file, err := os.Open(attachPath)
if err != nil {
    return fmt.Errorf("failed to open attachment %s: %w", attachPath, err)
}

content, err := io.ReadAll(file)
file.Close()  // Immediately closed after reading
```

**Analysis**:
- File handles are closed after reading
- Using `defer` would be slightly safer, but current pattern is correct

---

## 6. Dependencies

### 6.1 Dependency Analysis

| Package | Version | Known Vulnerabilities | Notes |
|---------|---------|----------------------|-------|
| `github.com/alecthomas/kong` | v1.14.0 | None known | CLI framework |
| `github.com/emersion/go-imap/v2` | v2.0.0-beta.8 | None known | Beta version, monitor for updates |
| `github.com/emersion/go-message` | v0.18.2 | None known | Stable |
| `github.com/zalando/go-keyring` | v0.2.6 | None known | Delegates to OS keyring |
| `golang.org/x/term` | v0.40.0 | None known | Go extended library |
| `gopkg.in/yaml.v3` | v3.0.1 | None known | Stable |

**Finding**: ACCEPTABLE

**Recommendations**:
- Monitor `go-imap/v2` for stable release (currently beta)
- Run `go mod tidy` and `govulncheck` periodically
- Consider using Dependabot or similar for dependency updates

---

## 7. Code Quality Issues

### 7.1 Ignored Errors

**Finding**: LOW RISK

```go
// In Close method
if err := c.client.Logout().Wait(); err != nil {
    // Ignore logout errors, just close
}
```

**Analysis**:
- Logout errors during close are intentionally ignored
- This is acceptable as the connection is being terminated anyway
- Comment explains the reasoning

### 7.2 Scanner Without Size Limit

**Finding**: LOW RISK

```go
scanner := bufio.NewScanner(os.Stdin)
var lines []string
for scanner.Scan() {
    lines = append(lines, scanner.Text())
}
```

**Analysis**:
- Used for reading email body from stdin
- Default scanner buffer is 64KB per line
- No practical concern for email body input
- Could add size limit for defensive programming

---

## 8. Summary of Findings

### Secure Practices Identified

1. **Credential storage**: Proper use of system keyring
2. **File permissions**: Correct restricted permissions on config files
3. **Password input**: Terminal password masking
4. **Input validation**: Message IDs and file paths validated
5. **Connection cleanup**: Proper defer patterns for resource cleanup
6. **Data exclusion**: Sensitive fields excluded from JSON output
7. **HTML sanitization**: Script/style removal from displayed content

### Items Requiring Attention

| Issue | Severity | Status |
|-------|----------|--------|
| TLS certificate verification disabled | Medium | Documented limitation |
| Password in memory as string | Low | Go language limitation |
| Beta dependency (go-imap v2) | Low | Monitor for updates |

### Recommendations

1. **Add localhost warning**: Warn users if IMAP/SMTP hosts are not localhost
2. **Certificate pinning**: Consider future support for Proton Bridge certificate pinning
3. **Dependency monitoring**: Set up automated vulnerability scanning
4. **Go module updates**: Update to stable go-imap when released
5. **Input size limits**: Add defensive size limits for stdin input

---

## Appendix: Files Reviewed

- `/home/bscott/src/pm-cli/cmd/pm-cli/main.go`
- `/home/bscott/src/pm-cli/internal/cli/cli.go`
- `/home/bscott/src/pm-cli/internal/cli/config.go`
- `/home/bscott/src/pm-cli/internal/cli/mail.go`
- `/home/bscott/src/pm-cli/internal/cli/mailbox.go`
- `/home/bscott/src/pm-cli/internal/config/config.go`
- `/home/bscott/src/pm-cli/internal/imap/client.go`
- `/home/bscott/src/pm-cli/internal/imap/types.go`
- `/home/bscott/src/pm-cli/internal/smtp/client.go`
- `/home/bscott/src/pm-cli/internal/output/formatter.go`
- `/home/bscott/src/pm-cli/go.mod`
- `/home/bscott/src/pm-cli/go.sum`
