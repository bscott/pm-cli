# AI Agent Integration

pm-cli is designed for integration with AI agents and automation tools.

## JSON Output

All commands support `--json` for machine-readable output:

```bash
pm-cli mail list --json
pm-cli mail read 123 --json
pm-cli mailbox list --json
pm-cli config doctor --json
```

## Command Schema

Get the full command schema as JSON:

```bash
pm-cli --help-json
```

This outputs a structured schema of all commands, flags, and their types - ideal for LLM tool definitions.

## Example Workflows

### List Unread Messages

```bash
pm-cli mail list --unread --json
```

Output:
```json
{
  "mailbox": "INBOX",
  "count": 3,
  "offset": 0,
  "limit": 20,
  "messages": [
    {
      "uid": 97,
      "seq_num": 97,
      "from": "sender@example.com",
      "subject": "Meeting tomorrow",
      "date": "2024-01-15 10:30",
      "date_iso": "2024-01-15T10:30:00Z",
      "seen": false,
      "flagged": true
    }
  ]
}
```

**Note:** The `date_iso` field provides RFC3339 timestamps for easier parsing by AI agents and automation tools.

### Read Message with Attachments

```bash
pm-cli mail read 123 --attachments --json
```

Output:
```json
{
  "message_id": "123",
  "attachments": [
    {
      "index": 0,
      "filename": "report.pdf",
      "content_type": "application/pdf",
      "size": 102400
    }
  ],
  "count": 1
}
```

### Send Email

```bash
pm-cli mail send -t user@example.com -s "Subject" -b "Body" --json
```

Output:
```json
{
  "success": true,
  "message": "Email sent successfully",
  "to": ["user@example.com"],
  "subject": "Subject"
}
```

### Search and Process

```bash
# Search for invoices from a specific sender
pm-cli mail search "invoice" --from accounting@company.com --json

# Download attachments from results
pm-cli mail download 456 0 -o invoice.pdf
```

### Check Configuration Health

```bash
pm-cli config doctor --json
```

Output:
```json
{
  "checks": [
    {"name": "Config file exists", "status": "ok"},
    {"name": "Config valid", "status": "ok"},
    {"name": "Email configured", "status": "ok", "message": "user@proton.me"},
    {"name": "Password in keyring", "status": "ok"},
    {"name": "IMAP port reachable", "status": "ok", "message": "127.0.0.1:1143"},
    {"name": "SMTP port reachable", "status": "ok", "message": "127.0.0.1:1025"},
    {"name": "IMAP login succeeds", "status": "ok"},
    {"name": "SMTP connection succeeds", "status": "ok"}
  ],
  "healthy": true
}
```

## Claude Code Integration

pm-cli works seamlessly with Claude Code. Example prompts:

- "Check my unread emails"
- "Send an email to X about Y"
- "Search for emails from my boss this week"
- "Download the attachment from email 123"
- "Reply to email 456 with 'Thanks!'"
- "Forward email 789 to the team"

## MCP Server (Future)

A Model Context Protocol server for pm-cli is planned, which will allow direct tool integration with Claude and other LLM interfaces.

## Error Handling

Errors are returned with appropriate exit codes:

| Exit Code | Meaning |
|-----------|---------|
| 0 | Success |
| 1 | General error |

JSON error output:
```json
{
  "error": "connection failed: dial tcp 127.0.0.1:1143: connect: connection refused"
}
```

## Rate Limiting

pm-cli connects to your local Proton Bridge instance, so there are no external API rate limits. However, be mindful of:

- Opening too many simultaneous IMAP connections
- Rapid-fire operations that might overwhelm the Bridge

## Security Considerations

- Passwords are stored in the system keyring, not in plain text
- IMAP/SMTP use TLS (via STARTTLS) even on localhost
- No credentials are logged or output in JSON responses
