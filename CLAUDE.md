# CLAUDE.md - pm-cli

## Project Overview

pm-cli is a Go CLI tool for interacting with ProtonMail via Proton Bridge IMAP/SMTP. It's designed for both terminal use and AI agent integration.

## Architecture

```
pm-cli/
├── cmd/pm-cli/main.go       # Entry point, Kong parser setup
├── internal/
│   ├── cli/                 # Command handlers
│   │   ├── cli.go           # Kong struct definitions
│   │   ├── config.go        # config init/show/set
│   │   ├── help.go          # JSON help generator
│   │   ├── mail.go          # mail list/read/send/delete/move/flag/search
│   │   ├── mailbox.go       # mailbox list/create/delete
│   │   └── version.go       # version command
│   ├── config/config.go     # YAML config + keyring storage
│   ├── imap/
│   │   ├── client.go        # IMAP client wrapper (go-imap v2)
│   │   └── types.go         # Message/Mailbox types
│   ├── output/formatter.go  # JSON/text output formatting
│   └── smtp/client.go       # SMTP client for sending
```

## Key Dependencies

- `github.com/alecthomas/kong` - CLI framework
- `github.com/emersion/go-imap/v2` - IMAP client
- `github.com/emersion/go-message` - Email parsing
- `github.com/zalando/go-keyring` - System keyring (libsecret)
- `gopkg.in/yaml.v3` - Config parsing

## Building

```bash
go build ./cmd/pm-cli
go install ./cmd/pm-cli
```

## Testing Connection

Requires Proton Bridge running:
```bash
systemctl --user start protonmail-bridge
pm-cli mailbox list
```

## Code Patterns

### Adding a New Command

1. Add struct to `internal/cli/cli.go`:
```go
type MailNewCmd struct {
    Flag string `help:"Description" short:"f"`
}
```

2. Add to parent command struct:
```go
type MailCmd struct {
    // ...
    New MailNewCmd `cmd:"" help:"Description"`
}
```

3. Implement Run method in appropriate file:
```go
func (c *MailNewCmd) Run(ctx *Context) error {
    // Implementation
}
```

4. Update `internal/cli/help.go` for JSON help schema.

### IMAP Operations

All IMAP operations go through `internal/imap/client.go`:
```go
client, _ := imap.NewClient(cfg)
client.Connect()
defer client.Close()
// Operations...
```

### Output Format

Use the formatter for consistent JSON/text output:
```go
if ctx.Formatter.JSON {
    return ctx.Formatter.PrintJSON(data)
}
fmt.Println(textOutput)
```

## Proton Bridge Notes

- Uses STARTTLS on port 1143 (IMAP) and 1025 (SMTP)
- Self-signed certificates require `InsecureSkipVerify: true`
- Password is Bridge-generated, not Proton account password
- Some emails are HTML-only; `htmlToText()` handles conversion

## Common Issues

- **TLS handshake error**: Bridge uses STARTTLS, not implicit TLS
- **Login failed**: Check Bridge password (not Proton password)
- **Bridge not running**: `systemctl --user start protonmail-bridge`
- **No body content**: Single-part HTML emails need special parsing

## Roadmap

See `1-Projects/pm-cli/pm-cli-roadmap.md` in Obsidian vault for full task list.

Priority items:
1. Attachment download
2. Reply/forward workflows
3. Batch operations
4. Config doctor command
