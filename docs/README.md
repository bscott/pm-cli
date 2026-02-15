# pm-cli Documentation

Command-line interface for ProtonMail via Proton Bridge.

## Contents

| Document | Description |
|----------|-------------|
| [Setup Guide](setup.md) | Installation and configuration |
| [Command Reference](commands.md) | Complete command documentation |
| [AI Agent Integration](ai-agents.md) | JSON output and automation |

## Quick Start

```bash
# Install
go install github.com/bscott/pm-cli/cmd/pm-cli@latest

# Configure (requires Proton Bridge running)
pm-cli config init

# Test connection
pm-cli config validate

# List emails
pm-cli mail list

# Read an email
pm-cli mail read 123

# Send an email
pm-cli mail send -t user@example.com -s "Subject" -b "Body"
```

## Command Overview

```
pm-cli
├── config
│   ├── init        # Setup wizard
│   ├── show        # Display config
│   ├── set         # Set values
│   ├── validate    # Test connection
│   └── doctor      # Run diagnostics
├── mail
│   ├── list        # List messages
│   ├── read        # Read message
│   ├── send        # Send email
│   ├── reply       # Reply to message
│   ├── forward     # Forward message
│   ├── delete      # Delete messages
│   ├── move        # Move to folder
│   ├── flag        # Manage flags
│   ├── search      # Search messages
│   └── download    # Save attachment
├── mailbox
│   ├── list        # List folders
│   ├── create      # Create folder
│   └── delete      # Delete folder
└── version         # Show version
```

## JSON Output

All commands support `--json` for machine-readable output:

```bash
pm-cli mail list --json
pm-cli config doctor --json
```

## Getting Help

```bash
pm-cli --help
pm-cli mail --help
pm-cli mail send --help
```
