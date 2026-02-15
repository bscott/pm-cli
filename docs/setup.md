# pm-cli Setup Guide

## Prerequisites

- [Proton Bridge](https://proton.me/mail/bridge) installed and running
- Go 1.21+ (for building from source)

## Installation

### From Source

```bash
go install github.com/bscott/pm-cli/cmd/pm-cli@latest
```

### Build Locally

```bash
git clone https://github.com/bscott/pm-cli.git
cd pm-cli
go build ./cmd/pm-cli
go install ./cmd/pm-cli
```

## Proton Bridge Setup

### 1. Install Proton Bridge

Download from [proton.me/mail/bridge](https://proton.me/mail/bridge)

### 2. Log In

Open Proton Bridge and log in to your Proton account.

### 3. Get Bridge Password

In Proton Bridge:
1. Click on your account
2. Go to **IMAP/SMTP settings**
3. Copy the **Bridge password** (not your Proton password)

### 4. Start Bridge Service (Linux)

```bash
# Enable and start the bridge service
systemctl --user enable --now protonmail-bridge

# Check status
systemctl --user status protonmail-bridge
```

## Configure pm-cli

Run the setup wizard:

```bash
pm-cli config init
```

This will prompt for:
- Email address (your ProtonMail address)
- IMAP host (default: 127.0.0.1)
- IMAP port (default: 1143)
- SMTP host (default: 127.0.0.1)
- SMTP port (default: 1025)
- Bridge password

## Verify Setup

Test your connection:

```bash
# Quick validation
pm-cli config validate

# Full diagnostics
pm-cli config doctor
```

List your mailboxes:

```bash
pm-cli mailbox list
```

## Configuration File

Config is stored at `~/.config/pm-cli/config.yaml`:

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

Password is stored securely in the system keyring:
- **Linux**: libsecret (GNOME Keyring, KWallet)
- **macOS**: Keychain
- **Windows**: Windows Credential Manager

## Troubleshooting

### "TLS handshake error"

Proton Bridge uses STARTTLS, not implicit TLS. This is handled automatically by pm-cli.

### "Login failed"

Make sure you're using the **Bridge password** from the Proton Bridge app, not your Proton account password.

### "Connection refused"

Check that Proton Bridge is running:

```bash
# Linux
systemctl --user status protonmail-bridge

# Or check the process
pgrep -f protonmail-bridge
```

### "Config doctor shows SMTP failure"

The SMTP test in `config doctor` may fail due to TLS differences, but actual sending usually works. Test with:

```bash
pm-cli mail send -t your-other-email@example.com -s "Test" -b "Test message"
```

### Reset Configuration

```bash
rm ~/.config/pm-cli/config.yaml
pm-cli config init
```

To also reset the stored password, use your system's keyring manager to remove the `pm-cli` entry.
