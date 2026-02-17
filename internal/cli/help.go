package cli

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type HelpSchema struct {
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	Description string          `json:"description"`
	Commands    []CommandSchema `json:"commands"`
	GlobalFlags []FlagSchema    `json:"global_flags"`
}

type CommandSchema struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Flags       []FlagSchema  `json:"flags,omitempty"`
	Args        []ArgSchema   `json:"args,omitempty"`
	Subcommands []CommandSchema `json:"subcommands,omitempty"`
	Examples    []string      `json:"examples,omitempty"`
}

type FlagSchema struct {
	Name        string `json:"name"`
	Short       string `json:"short,omitempty"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description"`
}

type ArgSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Description string `json:"description"`
}

func GenerateHelpJSON(cli *CLI) ([]byte, error) {
	schema := HelpSchema{
		Name:        "pm-cli",
		Version:     Version,
		Description: "ProtonMail CLI via Proton Bridge IMAP/SMTP",
		GlobalFlags: extractGlobalFlags(),
		Commands:    extractCommands(cli),
	}

	return json.MarshalIndent(schema, "", "  ")
}

func extractGlobalFlags() []FlagSchema {
	return []FlagSchema{
		{Name: "--json", Type: "bool", Description: "Output as JSON (applies to all commands)"},
		{Name: "--help-json", Type: "bool", Description: "Output command help as JSON (AI agent mode)"},
		{Name: "--config", Short: "-c", Type: "string", Description: "Path to config file"},
		{Name: "--verbose", Short: "-v", Type: "bool", Description: "Verbose output"},
		{Name: "--quiet", Short: "-q", Type: "bool", Description: "Suppress non-essential output"},
	}
}

func extractCommands(cli *CLI) []CommandSchema {
	return []CommandSchema{
		extractConfigCommands(),
		extractMailCommands(),
		extractMailboxCommands(),
		extractLabelCommands(),
		{
			Name:        "version",
			Description: "Show version information",
			Examples:    []string{"pm-cli version", "pm-cli version --json"},
		},
	}
}

func extractConfigCommands() CommandSchema {
	return CommandSchema{
		Name:        "config",
		Description: "Configuration management",
		Subcommands: []CommandSchema{
			{
				Name:        "config init",
				Description: "Interactive setup wizard for Proton Bridge connection",
				Examples:    []string{"pm-cli config init"},
			},
			{
				Name:        "config show",
				Description: "Display current configuration",
				Examples:    []string{"pm-cli config show", "pm-cli config show --json"},
			},
			{
				Name:        "config set",
				Description: "Set a configuration value",
				Args: []ArgSchema{
					{Name: "key", Type: "string", Required: true, Description: "Configuration key (e.g., bridge.email, defaults.limit)"},
					{Name: "value", Type: "string", Required: true, Description: "Value to set"},
				},
				Examples: []string{
					"pm-cli config set defaults.limit 50",
					"pm-cli config set defaults.format json",
					"pm-cli config set bridge.email user@protonmail.com",
				},
			},
		},
	}
}

func extractMailCommands() CommandSchema {
	return CommandSchema{
		Name:        "mail",
		Description: "Email operations",
		Subcommands: []CommandSchema{
			{
				Name:        "mail list",
				Description: "List messages in mailbox",
				Flags: []FlagSchema{
					{Name: "--mailbox", Short: "-m", Type: "string", Default: "INBOX", Description: "Mailbox name"},
					{Name: "--limit", Short: "-n", Type: "int", Default: "20", Description: "Number of messages to show"},
					{Name: "--unread", Type: "bool", Description: "Only show unread messages"},
				},
				Examples: []string{
					"pm-cli mail list",
					"pm-cli mail list --unread --json",
					"pm-cli mail list -m Sent -n 10",
				},
			},
			{
				Name:        "mail read",
				Description: "Read a specific message",
				Args: []ArgSchema{
					{Name: "id", Type: "string", Required: true, Description: "Message ID or sequence number"},
				},
				Flags: []FlagSchema{
					{Name: "--mailbox", Short: "-m", Type: "string", Description: "Mailbox name (defaults to configured mailbox)"},
					{Name: "--raw", Type: "bool", Description: "Show raw message"},
					{Name: "--headers", Type: "bool", Description: "Include all headers"},
					{Name: "--attachments", Type: "bool", Description: "List attachments only"},
					{Name: "--html", Type: "bool", Description: "Output HTML body instead of plain text"},
				},
				Examples: []string{
					"pm-cli mail read 123",
					"pm-cli mail read 123 -m Archive",
					"pm-cli mail read 123 --json",
					"pm-cli mail read 123 --raw",
				},
			},
			{
				Name:        "mail send",
				Description: "Compose and send email",
				Flags: []FlagSchema{
					{Name: "--to", Short: "-t", Type: "[]string", Required: true, Description: "Recipient(s)"},
					{Name: "--cc", Type: "[]string", Description: "CC recipients"},
					{Name: "--bcc", Type: "[]string", Description: "BCC recipients"},
					{Name: "--subject", Short: "-s", Type: "string", Required: true, Description: "Subject line"},
					{Name: "--body", Short: "-b", Type: "string", Description: "Body text (or use stdin)"},
					{Name: "--attach", Short: "-a", Type: "[]string", Description: "Attachment file paths"},
				},
				Examples: []string{
					"pm-cli mail send -t user@example.com -s 'Hello' -b 'Message body'",
					"echo 'Body from stdin' | pm-cli mail send -t user@example.com -s 'Hello'",
					"pm-cli mail send -t user@example.com -s 'With attachment' -a file.pdf",
				},
			},
			{
				Name:        "mail delete",
				Description: "Delete message(s)",
				Args: []ArgSchema{
					{Name: "ids", Type: "[]string", Required: true, Description: "Message ID(s) to delete"},
				},
				Flags: []FlagSchema{
					{Name: "--permanent", Type: "bool", Description: "Skip trash, delete permanently"},
				},
				Examples: []string{
					"pm-cli mail delete 123",
					"pm-cli mail delete 123 456 789",
					"pm-cli mail delete 123 --permanent",
				},
			},
			{
				Name:        "mail move",
				Description: "Move message to mailbox",
				Args: []ArgSchema{
					{Name: "id", Type: "string", Required: true, Description: "Message ID to move"},
					{Name: "mailbox", Type: "string", Required: true, Description: "Destination mailbox"},
				},
				Examples: []string{
					"pm-cli mail move 123 Archive",
					"pm-cli mail move 123 'Custom Folder'",
				},
			},
			{
				Name:        "mail flag",
				Description: "Manage message flags",
				Args: []ArgSchema{
					{Name: "id", Type: "string", Required: true, Description: "Message ID"},
				},
				Flags: []FlagSchema{
					{Name: "--read", Type: "bool", Description: "Mark as read"},
					{Name: "--unread", Type: "bool", Description: "Mark as unread"},
					{Name: "--star", Type: "bool", Description: "Add star"},
					{Name: "--unstar", Type: "bool", Description: "Remove star"},
				},
				Examples: []string{
					"pm-cli mail flag 123 --read",
					"pm-cli mail flag 123 --star",
					"pm-cli mail flag 123 --unread --unstar",
				},
			},
			{
				Name:        "mail search",
				Description: "Search messages",
				Args: []ArgSchema{
					{Name: "query", Type: "string", Required: true, Description: "Search query"},
				},
				Flags: []FlagSchema{
					{Name: "--mailbox", Short: "-m", Type: "string", Default: "INBOX", Description: "Mailbox to search"},
					{Name: "--from", Type: "string", Description: "Filter by sender"},
					{Name: "--subject", Type: "string", Description: "Filter by subject"},
					{Name: "--since", Type: "string", Description: "Messages since date (YYYY-MM-DD)"},
					{Name: "--before", Type: "string", Description: "Messages before date (YYYY-MM-DD)"},
				},
				Examples: []string{
					"pm-cli mail search 'meeting'",
					"pm-cli mail search 'invoice' --from accounts@example.com",
					"pm-cli mail search '' --since 2024-01-01 --json",
				},
			},
		},
	}
}

func extractMailboxCommands() CommandSchema {
	return CommandSchema{
		Name:        "mailbox",
		Description: "Mailbox management",
		Subcommands: []CommandSchema{
			{
				Name:        "mailbox list",
				Description: "List all mailboxes/folders",
				Examples:    []string{"pm-cli mailbox list", "pm-cli mailbox list --json"},
			},
			{
				Name:        "mailbox create",
				Description: "Create new mailbox",
				Args: []ArgSchema{
					{Name: "name", Type: "string", Required: true, Description: "Mailbox name to create"},
				},
				Examples: []string{"pm-cli mailbox create 'Work Projects'"},
			},
			{
				Name:        "mailbox delete",
				Description: "Delete mailbox",
				Args: []ArgSchema{
					{Name: "name", Type: "string", Required: true, Description: "Mailbox name to delete"},
				},
				Examples: []string{"pm-cli mailbox delete 'Old Folder'"},
			},
		},
	}
}

func extractLabelCommands() CommandSchema {
	return CommandSchema{
		Name:        "mail label",
		Description: "Manage message labels (Proton Mail labels via Bridge)",
		Subcommands: []CommandSchema{
			{
				Name:        "mail label list",
				Description: "List all available labels",
				Examples:    []string{"pm-cli mail label list", "pm-cli mail label list --json"},
			},
			{
				Name:        "mail label add",
				Description: "Add a label to message(s)",
				Args: []ArgSchema{
					{Name: "ids", Type: "[]string", Required: true, Description: "Message ID(s) to label"},
				},
				Flags: []FlagSchema{
					{Name: "--label", Short: "-l", Type: "string", Required: true, Description: "Label name to add"},
					{Name: "--mailbox", Short: "-m", Type: "string", Default: "INBOX", Description: "Source mailbox"},
				},
				Examples: []string{
					"pm-cli mail label add 123 -l Important",
					"pm-cli mail label add 123 456 -l 'Work/Projects'",
					"pm-cli mail label add 123 -l Todo -m Archive",
				},
			},
			{
				Name:        "mail label remove",
				Description: "Remove a label from message(s)",
				Args: []ArgSchema{
					{Name: "ids", Type: "[]string", Required: true, Description: "Message ID(s) to unlabel"},
				},
				Flags: []FlagSchema{
					{Name: "--label", Short: "-l", Type: "string", Required: true, Description: "Label name to remove"},
				},
				Examples: []string{
					"pm-cli mail label remove 123 -l Important",
					"pm-cli mail label remove 123 456 -l Todo",
				},
			},
		},
	}
}

// extractFieldsFromStruct extracts flag/arg information from struct tags using reflection
// This is used for dynamic introspection when needed
func extractFieldsFromStruct(t reflect.Type) []FlagSchema {
	var flags []FlagSchema

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip embedded structs
		if field.Anonymous {
			continue
		}

		// Check for kong tags
		helpTag := field.Tag.Get("help")
		shortTag := field.Tag.Get("short")
		defaultTag := field.Tag.Get("default")
		requiredTag := field.Tag.Get("required")

		if helpTag == "" {
			continue
		}

		flagName := "--" + strings.ToLower(field.Name)
		if nameTag := field.Tag.Get("name"); nameTag != "" {
			flagName = "--" + nameTag
		}

		flag := FlagSchema{
			Name:        flagName,
			Type:        getTypeString(field.Type),
			Description: helpTag,
			Default:     defaultTag,
			Required:    requiredTag == "true" || requiredTag == "",
		}

		if shortTag != "" {
			flag.Short = shortTag
		}

		flags = append(flags, flag)
	}

	return flags
}

func getTypeString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Bool:
		return "bool"
	case reflect.Slice:
		return "[]" + getTypeString(t.Elem())
	default:
		return t.String()
	}
}

func PrintHelpJSON(cli *CLI) error {
	data, err := GenerateHelpJSON(cli)
	if err != nil {
		return fmt.Errorf("failed to generate help JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}
