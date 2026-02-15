# pm-cli Command Reference

Complete reference for all pm-cli commands.

## Global Flags

These flags are available on all commands:

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--help-json` | Output command schema as JSON (for AI agents) |
| `-c, --config` | Path to config file |
| `-v, --verbose` | Verbose output |
| `-q, --quiet` | Suppress non-essential output |

---

## config

Configuration management commands.

### config init

Interactive setup wizard for configuring pm-cli.

```bash
pm-cli config init
```

Prompts for:
- ProtonMail email address
- IMAP host and port (default: 127.0.0.1:1143)
- SMTP host and port (default: 127.0.0.1:1025)
- Bridge password (stored securely in system keyring)

### config show

Display current configuration.

```bash
pm-cli config show
pm-cli config show --json
```

### config set

Set a configuration value.

```bash
pm-cli config set <key> <value>
```

**Keys:**
- `bridge.imap_host` - IMAP server hostname
- `bridge.imap_port` - IMAP server port
- `bridge.smtp_host` - SMTP server hostname
- `bridge.smtp_port` - SMTP server port
- `bridge.email` - Email address
- `defaults.mailbox` - Default mailbox (e.g., INBOX)
- `defaults.limit` - Default message limit
- `defaults.format` - Output format (text/json)

**Examples:**
```bash
pm-cli config set defaults.limit 50
pm-cli config set defaults.format json
```

### config validate

Test connection to Proton Bridge.

```bash
pm-cli config validate
pm-cli config validate --json
```

Returns success if IMAP connection and authentication work.

### config doctor

Run comprehensive diagnostics on your configuration.

```bash
pm-cli config doctor
pm-cli config doctor --json
```

**Checks performed:**
1. Config file exists
2. Config file is valid YAML
3. Email is configured
4. Password exists in keyring
5. IMAP port is reachable
6. SMTP port is reachable
7. IMAP login succeeds
8. SMTP connection succeeds

---

## mail

Email operations.

### mail list

List messages in a mailbox.

```bash
pm-cli mail list [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-m, --mailbox` | Mailbox name | INBOX |
| `-n, --limit` | Number of messages | 20 |
| `--offset` | Skip first N messages | 0 |
| `-p, --page` | Page number (1-based) | 0 |
| `--unread` | Only show unread messages | false |

**Pagination:**
- Use `--offset` to skip messages (e.g., `--offset 20` skips the 20 most recent)
- Use `--page` for page-based navigation (e.g., `-p 2 -n 20` shows messages 21-40)
- JSON output includes `offset`, `limit`, and `page` fields

**Examples:**
```bash
pm-cli mail list
pm-cli mail list -n 50
pm-cli mail list -m Sent
pm-cli mail list --unread
pm-cli mail list --json

# Pagination
pm-cli mail list --offset 20           # Skip 20 most recent
pm-cli mail list -p 2 -n 20            # Page 2 (messages 21-40)
pm-cli mail list -p 3 -n 10 --json     # Page 3, 10 per page, JSON output
```

### mail read

Read a specific message.

```bash
pm-cli mail read <id> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--raw` | Show raw MIME source |
| `--headers` | Include all headers |
| `--attachments` | List attachments only |
| `--html` | Output HTML body instead of plain text |

**Examples:**
```bash
pm-cli mail read 123
pm-cli mail read 123 --headers
pm-cli mail read 123 --raw
pm-cli mail read 123 --html            # View HTML content
pm-cli mail read 123 --attachments
pm-cli mail read 123 --json
```

### mail send

Compose and send an email.

```bash
pm-cli mail send [flags]
```

**Flags:**
| Flag | Description | Required |
|------|-------------|----------|
| `-t, --to` | Recipient(s) | Yes |
| `--cc` | CC recipients | No |
| `--bcc` | BCC recipients | No |
| `-s, --subject` | Subject line | Yes |
| `-b, --body` | Body text | No* |
| `-a, --attach` | Attachments | No |

*Body can be provided via stdin if not specified.

**Examples:**
```bash
pm-cli mail send -t user@example.com -s "Hello" -b "Message body"
pm-cli mail send -t user@example.com -s "Report" -a report.pdf
echo "Body text" | pm-cli mail send -t user@example.com -s "Subject"
pm-cli mail send -t a@example.com -t b@example.com -s "Group email" -b "Hi all"
```

### mail reply

Reply to a message.

```bash
pm-cli mail reply <id> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--all` | Reply to all recipients |
| `-b, --body` | Reply body |
| `-a, --attach` | Attachments |

**Examples:**
```bash
pm-cli mail reply 123 -b "Thanks for the info!"
pm-cli mail reply 123 --all -b "Confirming receipt."
echo "Reply text" | pm-cli mail reply 123
```

The reply includes:
- Proper `Re:` subject prefix (avoids `Re: Re:` stacking)
- `In-Reply-To` header for threading
- `References` header for threading
- Quoted original message with `>` prefix

### mail forward

Forward a message.

```bash
pm-cli mail forward <id> [flags]
```

**Flags:**
| Flag | Description | Required |
|------|-------------|----------|
| `-t, --to` | Recipient(s) | Yes |
| `-b, --body` | Additional message | No |
| `-a, --attach` | Additional attachments | No |

**Examples:**
```bash
pm-cli mail forward 123 -t colleague@example.com
pm-cli mail forward 123 -t boss@example.com -b "FYI - see below"
pm-cli mail forward 123 -t user@example.com -a extra-doc.pdf
```

The forwarded message includes:
- `Fwd:` subject prefix
- Forwarded message header block (From, Date, Subject, To)
- Original message body

### mail delete

Delete messages.

```bash
pm-cli mail delete <id>... [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--permanent` | Skip trash, delete permanently |

**Examples:**
```bash
pm-cli mail delete 123
pm-cli mail delete 123 124 125
pm-cli mail delete 123 --permanent
```

### mail move

Move a message to another mailbox.

```bash
pm-cli mail move <id> <mailbox>
```

**Examples:**
```bash
pm-cli mail move 123 Archive
pm-cli mail move 123 "Projects/Active"
```

### mail flag

Manage message flags.

```bash
pm-cli mail flag <id> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--read` | Mark as read |
| `--unread` | Mark as unread |
| `--star` | Add star |
| `--unstar` | Remove star |

**Examples:**
```bash
pm-cli mail flag 123 --read
pm-cli mail flag 123 --unread
pm-cli mail flag 123 --star
pm-cli mail flag 123 --read --star
```

### mail search

Search messages.

```bash
pm-cli mail search <query> [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-m, --mailbox` | Mailbox to search | INBOX |
| `--from` | Filter by sender | |
| `--subject` | Filter by subject | |
| `--since` | Messages since date (YYYY-MM-DD) | |
| `--before` | Messages before date (YYYY-MM-DD) | |

**Examples:**
```bash
pm-cli mail search "invoice"
pm-cli mail search "" --from boss@example.com
pm-cli mail search "" --since 2024-01-01
pm-cli mail search "project" --from client@example.com --since 2024-06-01
```

### mail download

Download an attachment.

```bash
pm-cli mail download <id> <index> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-o, --out` | Output path (default: original filename) |

**Examples:**
```bash
# First, list attachments
pm-cli mail read 123 --attachments

# Then download by index
pm-cli mail download 123 0
pm-cli mail download 123 0 -o ~/Downloads/report.pdf
```

### mail draft

Manage email drafts.

#### mail draft list

List all drafts.

```bash
pm-cli mail draft list
pm-cli mail draft list -n 50 --json
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-n, --limit` | Number of drafts to show (default: 20) |

#### mail draft create

Create a new draft.

```bash
pm-cli mail draft create [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-t, --to` | Recipient(s) |
| `--cc` | CC recipients |
| `-s, --subject` | Subject line |
| `-b, --body` | Body text |
| `-a, --attach` | Attachments |

**Examples:**
```bash
pm-cli mail draft create -t user@example.com -s "Meeting notes" -b "Draft content..."
pm-cli mail draft create -s "Notes" <<< "Body from stdin"
```

#### mail draft edit

Edit an existing draft.

```bash
pm-cli mail draft edit <id> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-t, --to` | New recipient(s) |
| `--cc` | New CC recipients |
| `-s, --subject` | New subject line |
| `-b, --body` | New body text |
| `-a, --attach` | New attachments |

**Examples:**
```bash
# Update subject
pm-cli mail draft edit 42 -s "Updated subject"

# Update body
pm-cli mail draft edit 42 -b "New content"
```

#### mail draft delete

Delete draft(s).

```bash
pm-cli mail draft delete <id>...
```

**Examples:**
```bash
pm-cli mail draft delete 42
pm-cli mail draft delete 42 43 44
```

---

## mailbox

Mailbox/folder management.

### mailbox list

List all mailboxes/folders.

```bash
pm-cli mailbox list
pm-cli mailbox list --json
```

### mailbox create

Create a new mailbox.

```bash
pm-cli mailbox create <name>
```

**Examples:**
```bash
pm-cli mailbox create "Projects"
pm-cli mailbox create "Archive/2024"
```

### mailbox delete

Delete a mailbox.

```bash
pm-cli mailbox delete <name>
```

**Examples:**
```bash
pm-cli mailbox delete "Old Folder"
```

---

## version

Show version information.

```bash
pm-cli version
pm-cli version --json
```
