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
pm-cli mail list --json             # JSON output
```

### Read Messages

```bash
pm-cli mail read 123                # Read message #123
pm-cli mail read 123 --json         # JSON output with body
pm-cli mail read 123 --headers      # Include all headers
pm-cli mail read 123 --raw          # Raw MIME source
```

### Send Email

```bash
pm-cli mail send -t user@example.com -s "Subject" -b "Body text"
pm-cli mail send -t user@example.com -s "Subject" -a attachment.pdf
echo "Body from stdin" | pm-cli mail send -t user@example.com -s "Subject"
```

### Manage Messages

```bash
pm-cli mail delete 123              # Move to trash
pm-cli mail delete 123 --permanent  # Delete permanently
pm-cli mail move 123 Archive        # Move to folder
pm-cli mail flag 123 --read         # Mark as read
pm-cli mail flag 123 --star         # Add star
```

### Search

```bash
pm-cli mail search "invoice"                    # Search body
pm-cli mail search "" --from boss@example.com   # Filter by sender
pm-cli mail search "" --since 2024-01-01        # Filter by date
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
pm-cli config set defaults.format json
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
      "seen": false,
      "flagged": true
    }
  ]
}
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
