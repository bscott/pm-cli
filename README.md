# pm-cli

A command-line interface for ProtonMail via Proton Bridge IMAP/SMTP. Designed for terminal use and AI agent integration.

## Installation

```bash
go install github.com/bscott/pm-cli/cmd/pm-cli@latest
```

## Prerequisites

- [Proton Bridge](https://proton.me/mail/bridge) running and logged in
- Go 1.21+

## Setup

1. Start Proton Bridge and log in to your account
2. Get your Bridge password from the Bridge app (Account → IMAP/SMTP settings)
3. Run the setup wizard:

```bash
pm-cli config init
```

## Usage

### List Messages

```bash
pm-cli mail list                    # Latest 20 messages in INBOX
pm-cli mail list -n 50              # Latest 50 messages
pm-cli mail list -m Sent            # Messages in Sent folder
pm-cli mail list --unread           # Only unread messages
pm-cli mail list --offset 20        # Skip first 20 messages
pm-cli mail list -p 2 -n 20         # Page 2 (messages 21-40)
pm-cli mail list --json             # JSON output
```

### Read Messages

```bash
pm-cli mail read 123                # Read message #123
pm-cli mail read 123 -m Archive     # Read from a specific mailbox
pm-cli mail read 123 --json         # JSON output with body
pm-cli mail read 123 --headers      # Include all headers
pm-cli mail read 123 --html         # Output HTML body
pm-cli mail read 123 --attachments  # List attachments
pm-cli mail read 123 --raw          # Raw MIME source
```

### Send Email

```bash
pm-cli mail send -t user@example.com -s "Subject" -b "Body text"
pm-cli mail send -t user@example.com -s "Subject" -a attachment.pdf
pm-cli mail send -t user@example.com --template welcome.yaml -V name=Alice
echo "Body from stdin" | pm-cli mail send -t user@example.com -s "Subject"
```

### Reply & Forward

```bash
pm-cli mail reply 123 -b "Thanks for the info"
pm-cli mail reply 123 --all -b "Reply to all"
pm-cli mail forward 123 -t other@example.com -b "FYI"
```

### Attachments

```bash
pm-cli mail read 123 --attachments  # List attachments with indices
pm-cli mail download 123 0          # Download first attachment
pm-cli mail download 123 0 -o ~/Downloads/file.pdf
```

### Drafts

```bash
pm-cli mail draft list              # List all drafts
pm-cli mail draft create -t user@example.com -s "Subject" -b "Draft body"
pm-cli mail draft edit 456 -b "Updated body"
pm-cli mail draft delete 456
```

### Thread/Conversation

```bash
pm-cli mail thread 123              # Show full conversation thread
```

### Watch for New Mail

```bash
pm-cli mail watch                           # Watch INBOX, poll every 30s
pm-cli mail watch -m INBOX -i 60            # Poll every 60 seconds
pm-cli mail watch --exec "notify-send 'New mail: {}'"  # Run command on new mail
pm-cli mail watch --once                    # Exit after first new message
```

### Manage Messages

```bash
pm-cli mail delete 123              # Move to trash
pm-cli mail delete 123 456 789      # Batch delete
pm-cli mail delete --query "from:spam@example.com"  # Delete by search
pm-cli mail delete 123 --permanent  # Delete permanently
pm-cli mail move 123 Archive        # Move to folder
pm-cli mail move 123 456 -d Archive # Batch move
pm-cli mail flag 123 --read         # Mark as read
pm-cli mail flag 123 --star         # Add star
pm-cli mail flag 123 456 --unread   # Batch flag
```

### Labels

```bash
pm-cli mail label list              # List available labels
pm-cli mail label add 123 -l Important
pm-cli mail label add 123 456 -l "Work/Projects"
pm-cli mail label remove 123 -l Important
```

### Search

```bash
pm-cli mail search "invoice"                        # Search body text
pm-cli mail search --from boss@example.com          # Filter by sender
pm-cli mail search --subject "meeting"              # Filter by subject
pm-cli mail search --since 2024-01-01               # Messages since date
pm-cli mail search --before 2024-12-31              # Messages before date
pm-cli mail search --has-attachments                # Only with attachments
pm-cli mail search --larger-than 1M                 # Size filters
pm-cli mail search --from user@example.com --or --subject "urgent"  # Boolean OR
pm-cli mail search --from spam@example.com --not    # Negate search
```

### Contacts

```bash
pm-cli contacts list                # List all contacts
pm-cli contacts search "alice"      # Search by name or email
pm-cli contacts add alice@example.com -n "Alice Smith"
pm-cli contacts remove alice@example.com
```

### Mailbox Management

```bash
pm-cli mailbox list                 # List all mailboxes
pm-cli mailbox create "Projects"    # Create mailbox
pm-cli mailbox delete "Old Folder"  # Delete mailbox
```

### Configuration

```bash
pm-cli config show                  # Display current config
pm-cli config set defaults.limit 50 # Set default limit
pm-cli config validate              # Test Bridge connection
pm-cli config doctor                # Run diagnostics (8 checks)
```

## AI Agent Integration

pm-cli is designed for AI agent workflows with machine-readable output:

```bash
# Get full command schema for agents
pm-cli --help-json

# All commands support JSON output
pm-cli mail list --json
pm-cli mail read 123 --json
pm-cli mailbox list --json
```

### Semantic Commands for AI Processing

```bash
# Get AI-friendly email summary
pm-cli mail summarize 123 --json
# Returns: sentiment, priority, action_required, summary

# Extract structured data from email
pm-cli mail extract 123 --json
# Returns: emails, URLs, dates, phone numbers, action items
```

### Idempotency for Safe Automation

```bash
# Prevent duplicate sends with idempotency keys
pm-cli mail send -t user@example.com -s "Alert" -b "..." --idempotency-key "alert-2024-01-15"
pm-cli mail reply 123 -b "..." --idempotency-key "reply-123-v1"
```

### JSON Output Example

```json
{
  "mailbox": "INBOX",
  "count": 3,
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

## Email Templates

Create reusable templates with YAML frontmatter:

```yaml
# templates/welcome.yaml
---
subject: "Welcome to {{company}}, {{name}}!"
to:
  - "{{email}}"
cc:
  - onboarding@company.com
---
Hi {{name}},

Welcome to {{company}}! We're excited to have you.

Best regards,
The Team
```

Use with variables:

```bash
pm-cli mail send --template welcome.yaml -V name=Alice -V email=alice@example.com -V company=Acme
```

## Configuration

Config file: `~/.config/pm-cli/config.yaml`

```yaml
bridge:
  imap_host: 127.0.0.1
  imap_port: 1143
  smtp_host: 127.0.0.1
  smtp_port: 1025
  email: user@protonmail.com

defaults:
  mailbox: INBOX
  limit: 20
  format: text
```

Password is stored securely in the system keyring (libsecret on Linux).

## Global Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--help-json` | Full command schema as JSON |
| `-c, --config` | Path to config file |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Suppress non-essential output |
| `--no-color` | Disable colored output (also respects `NO_COLOR` env) |

## Proton Bridge Setup

1. Download [Proton Bridge](https://proton.me/mail/bridge)
2. Log in to your Proton account
3. Enable and start the service:

```bash
# Linux (systemd)
systemctl --user enable --now protonmail-bridge
```

## Platform Support

pm-cli works on **Linux, macOS, and Windows** - anywhere Proton Bridge runs. As long as Proton Bridge is installed, logged in, and running, pm-cli will connect to it.

| Platform | Keyring Backend |
|----------|-----------------|
| Linux | libsecret (GNOME Keyring, KWallet) |
| macOS | Keychain |
| Windows | Windows Credential Manager |

## License

MIT
