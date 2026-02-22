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

If SMTP port reachability fails in check 6, check 8 is reported as `cannot test - SMTP port not reachable` and SMTP auth is skipped.

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
| `-m, --mailbox` | Mailbox name (defaults to configured mailbox) |
| `--raw` | Show raw MIME source |
| `--headers` | Include all headers |
| `--attachments` | List attachments only |
| `--html` | Output HTML body instead of plain text |

**Examples:**
```bash
pm-cli mail read 123
pm-cli mail read 123 -m Archive
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
| `-t, --to` | Recipient(s) | No* |
| `--cc` | CC recipients | No |
| `--bcc` | BCC recipients | No |
| `-s, --subject` | Subject line | No* |
| `-b, --body` | Body text | No* |
| `-a, --attach` | Attachments | No |
| `--template` | Template file path | No |
| `-V` | Template variables (key=value) | No |
| `--idempotency-key` | Unique key to prevent duplicate sends | No |

*Required unless provided via template. Body can also be provided via stdin.

**Idempotency:** Use `--idempotency-key` to prevent duplicate emails when retrying failed operations. Keys are valid for 24 hours.

**Templates:** Use `--template` to load email content from a template file. Templates use YAML frontmatter for headers (to, cc, bcc, subject) and the rest is the body. Use `-V key=value` to substitute `{{key}}` placeholders.

**Template Format:**
```yaml
---
to: {{recipient}}
cc: manager@example.com
subject: Weekly Report - {{date}}
---
Hi {{name}},

Here is the weekly report for {{date}}.

Best regards
```

**Examples:**
```bash
pm-cli mail send -t user@example.com -s "Hello" -b "Message body"
pm-cli mail send -t user@example.com -s "Report" -a report.pdf
echo "Body text" | pm-cli mail send -t user@example.com -s "Subject"
pm-cli mail send -t a@example.com -t b@example.com -s "Group email" -b "Hi all"

# With idempotency key (for AI agents)
pm-cli mail send -t user@example.com -s "Order confirmation" --idempotency-key "order-12345"

# Using templates
pm-cli mail send --template report.tmpl -V recipient=user@example.com -V name=John -V date=2024-01-15

# Template with command-line override (--to overrides template's to)
pm-cli mail send --template newsletter.tmpl -t override@example.com
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
| `--idempotency-key` | Unique key to prevent duplicate sends |

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
| `--idempotency-key` | Unique key to prevent duplicate sends | No |

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

### mail watch

Watch a mailbox for new messages.

```bash
pm-cli mail watch [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-m, --mailbox` | Mailbox to watch | INBOX |
| `-i, --interval` | Poll interval in seconds | 30 |
| `--unread` | Only notify for unread messages | true |
| `-e, --exec` | Command to execute on new mail (use {} for message ID) | |
| `--once` | Exit after first new message | false |

**Examples:**
```bash
# Watch INBOX with default settings
pm-cli mail watch

# Watch Sent folder every 60 seconds
pm-cli mail watch -m Sent -i 60

# Execute a command when new mail arrives
pm-cli mail watch -e "notify-send 'New mail' 'Message ID: {}'"

# Wait for one new message then exit
pm-cli mail watch --once

# Watch with JSON output (for scripts/agents)
pm-cli mail watch --json

# Notify and read the new message
pm-cli mail watch -e "pm-cli mail read {}"
```

The watch command:
- Polls the mailbox at regular intervals
- Tracks message UIDs to detect new arrivals
- Optionally executes a command with the message ID substituted for `{}`
- Handles Ctrl+C gracefully for clean shutdown
- Supports JSON output for integration with scripts and AI agents

---

### mail thread

Show conversation thread for a message.

```bash
pm-cli mail thread <id> [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-m, --mailbox` | Mailbox to search | INBOX |

**Examples:**
```bash
pm-cli mail thread 123
pm-cli mail thread 123 -m Sent
pm-cli mail thread 123 --json
```

The thread command:
- Finds all messages in the same conversation
- Matches by subject (with Re:/Fwd: prefixes stripped)
- Displays messages in chronological order
- Shows body text (truncated) for each message

---

### mail label

Manage message labels. Proton Mail labels are exposed via Bridge as IMAP folders under `Labels/`.

**How it works:**
- Labels appear as folders under `Labels/` (e.g., `Labels/Important`, `Labels/Work`)
- Adding a label copies the message to the label folder
- Removing a label deletes the message from the label folder (but keeps it in INBOX/Archive)
- Create new labels in the Proton Mail web interface or mobile app

#### mail label list

List all available labels.

```bash
pm-cli mail label list
pm-cli mail label list --json
```

#### mail label add

Add a label to message(s).

```bash
pm-cli mail label add <id>... [flags]
```

**Flags:**
| Flag | Description | Required |
|------|-------------|----------|
| `-l, --label` | Label name to add | Yes |
| `-m, --mailbox` | Source mailbox | No (default: INBOX) |

**Examples:**
```bash
# Add a single label
pm-cli mail label add 123 -l Important

# Add label to multiple messages
pm-cli mail label add 123 456 789 -l "Work/Projects"

# Add label to a message in a different mailbox
pm-cli mail label add 123 -l Todo -m Archive
```

#### mail label remove

Remove a label from message(s).

```bash
pm-cli mail label remove <id>... [flags]
```

**Flags:**
| Flag | Description | Required |
|------|-------------|----------|
| `-l, --label` | Label name to remove | Yes |

**Important:** The message IDs must be from within the label folder itself. To find the correct IDs, first list messages in the label folder:

```bash
pm-cli mail list -m "Labels/Important"
```

Then use those IDs to remove the label:

```bash
pm-cli mail label remove 5 -l Important
```

**Examples:**
```bash
# Remove a label
pm-cli mail label remove 123 -l Important

# Remove label from multiple messages
pm-cli mail label remove 123 456 -l Todo
```

**Limitations:**
- Labels must be created in the Proton Mail web/mobile interface (IMAP folder creation under Labels/ is not supported by Bridge)
- IMAP keywords (X-Keywords header) are not synchronized by Bridge - only folder-based labels work

### mail summarize

Summarize a message in structured JSON format for AI processing.

```bash
pm-cli mail summarize <id> [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-m, --mailbox` | Mailbox name | INBOX |

**Output includes:**
- Message metadata (from, to, cc, subject, date)
- Read/flagged status
- Body preview (first 500 chars)
- Attachment count and details

**Examples:**
```bash
pm-cli mail summarize 123
pm-cli mail summarize 123 -m Sent
```

### mail extract

Extract structured data from a message for AI processing.

```bash
pm-cli mail extract <id> [flags]
```

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `-m, --mailbox` | Mailbox name | INBOX |

**Extracts:**
- Email addresses mentioned in body
- URLs/links
- Dates mentioned in text
- Phone numbers
- Action items (bulleted/numbered lists)
- Attachment info

**Examples:**
```bash
pm-cli mail extract 123
pm-cli mail extract 123 -m Archive
```

**Example output:**
```json
{
  "id": 123,
  "subject": "Meeting Follow-up",
  "from": "sender@example.com",
  "mentioned_emails": ["john@example.com", "jane@example.com"],
  "urls": ["https://meet.google.com/abc-defg-hij"],
  "mentioned_dates": ["January 15, 2024", "2024-01-20"],
  "action_items": ["Review the proposal", "Send feedback by Friday"]
}
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

## contacts

Address book management. Contacts are stored locally in `~/.config/pm-cli/contacts.json`.

### contacts list

List all contacts in the address book.

```bash
pm-cli contacts list
pm-cli contacts list --json
```

### contacts search

Search contacts by name or email.

```bash
pm-cli contacts search <query>
```

**Examples:**
```bash
pm-cli contacts search john
pm-cli contacts search example.com
pm-cli contacts search "John Doe" --json
```

### contacts add

Add a contact to the address book.

```bash
pm-cli contacts add <email> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `-n, --name` | Contact display name |

**Examples:**
```bash
pm-cli contacts add user@example.com
pm-cli contacts add user@example.com --name "John Doe"
pm-cli contacts add user@example.com -n "Jane Smith"
```

### contacts remove

Remove a contact from the address book.

```bash
pm-cli contacts remove <email>
```

**Examples:**
```bash
pm-cli contacts remove user@example.com
pm-cli contacts remove user@example.com --json
```

---

## version

Show version information.

```bash
pm-cli version
pm-cli version --json
```
