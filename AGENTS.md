# AGENTS.md - AI Agent Integration Guide

## Overview

pm-cli is designed for AI agent workflows. All commands support `--json` output and the full command schema is available via `--help-json`.

## Quick Start for Agents

```bash
# Get full command schema
pm-cli --help-json

# All operations return JSON
pm-cli mail list --json
pm-cli mail read 123 --json
pm-cli mailbox list --json
```

## Command Schema

Retrieve the complete command schema:

```bash
pm-cli --help-json
```

Returns:
```json
{
  "name": "pm-cli",
  "version": "0.1.0",
  "description": "ProtonMail CLI via Proton Bridge IMAP/SMTP",
  "commands": [...],
  "global_flags": [...]
}
```

## Common Workflows

### List Unread Messages

```bash
pm-cli mail list --unread --json
```

Response:
```json
{
  "mailbox": "INBOX",
  "count": 5,
  "messages": [
    {
      "uid": 123,
      "seq_num": 123,
      "from": "sender@example.com",
      "subject": "Important update",
      "date": "2024-01-15 10:30",
      "seen": false,
      "flagged": false
    }
  ]
}
```

### Read Message Content

```bash
pm-cli mail read 123 --json
```

Response:
```json
{
  "uid": 123,
  "seq_num": 123,
  "message_id": "<abc@example.com>",
  "from": "sender@example.com",
  "to": ["recipient@example.com"],
  "subject": "Important update",
  "date": "2024-01-15 10:30:00",
  "flags": ["\\Seen"],
  "body": "Plain text content...",
  "html_body": "<html>..."
}
```

### Send Email

```bash
pm-cli mail send -t recipient@example.com -s "Subject" -b "Body" --json
```

Response:
```json
{
  "success": true,
  "message": "Email sent successfully",
  "to": ["recipient@example.com"],
  "subject": "Subject"
}
```

### Search Messages

```bash
pm-cli mail search "invoice" --from accounts@example.com --json
```

### Manage Flags

```bash
pm-cli mail flag 123 --read --json
pm-cli mail flag 123 --star --json
```

### Move Messages

```bash
pm-cli mail move 123 Archive --json
```

### Delete Messages

```bash
pm-cli mail delete 123 --json
pm-cli mail delete 123 --permanent --json
```

## Error Handling

Errors return JSON with `success: false`:

```json
{
  "success": false,
  "error": "IMAP login failed: authentication error"
}
```

Exit codes:
- `0`: Success
- `1`: Error (check JSON error field)

## Message IDs

Messages are identified by sequence number (`seq_num`), which is the ID shown in `mail list`. Use this number for `mail read`, `mail delete`, `mail move`, and `mail flag`.

Note: Sequence numbers can change when messages are deleted. For persistent identification, use the `uid` field.

## Mailbox Names

Common Proton Bridge mailboxes:
- `INBOX` - Primary inbox
- `Sent` - Sent messages
- `Drafts` - Draft messages
- `Trash` - Deleted messages
- `Spam` - Spam folder
- `Archive` - Archived messages
- `All Mail` - All messages
- `Starred` - Flagged messages
- `Labels/*` - Proton labels
- `Folders/*` - Custom folders

List all with:
```bash
pm-cli mailbox list --json
```

## Semantic Commands

Commands designed specifically for AI/LLM processing:

### Summarize Email

```bash
pm-cli mail summarize 123 --json
```

Response:
```json
{
  "id": "123",
  "from": "sender@example.com",
  "subject": "Meeting Request",
  "summary": "Request to schedule a meeting next Tuesday at 2pm",
  "sentiment": "neutral",
  "priority": "normal",
  "action_required": true
}
```

### Extract Structured Data

```bash
pm-cli mail extract 123 --json
```

Response:
```json
{
  "emails": ["alice@example.com", "bob@example.com"],
  "urls": ["https://example.com/doc"],
  "dates": ["2024-01-18", "next Tuesday"],
  "phone_numbers": ["+1-555-123-4567"],
  "action_items": ["Review document", "Confirm attendance"]
}
```

## Idempotency

Prevent duplicate sends with idempotency keys:

```bash
pm-cli mail send -t user@example.com -s "Alert" -b "..." --idempotency-key "alert-2024-01-15" --json
```

Keys are stored locally with 24-hour TTL. Duplicate key returns error without sending.

## RFC3339 Timestamps

JSON output includes `date_iso` field for consistent parsing:

```json
{
  "date": "2024-01-15 10:30",
  "date_iso": "2024-01-15T10:30:00Z"
}
```

## Best Practices

1. **Always use `--json`** for programmatic access
2. **Check `success` field** in responses
3. **Handle rate limiting** - add delays between rapid requests
4. **Use `uid` for persistence** - `seq_num` changes on delete
5. **Use `date_iso`** for date parsing (RFC3339 format)
6. **Use idempotency keys** for send operations to prevent duplicates
7. **Use semantic commands** for email triage and data extraction

## Prerequisites

1. Proton Bridge must be running
2. Account must be logged in via Bridge
3. pm-cli must be configured: `pm-cli config init`

## Testing Connection

```bash
pm-cli mailbox list --json
```

If this returns mailboxes, the connection is working.
