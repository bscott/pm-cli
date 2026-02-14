package cli

import (
	"github.com/bscott/pm-cli/internal/config"
	"github.com/bscott/pm-cli/internal/output"
)

var Version = "0.1.0"

type Globals struct {
	JSON     bool   `help:"Output as JSON" name:"json"`
	HelpJSON bool   `help:"Output command help as JSON (AI agent mode)" name:"help-json"`
	Config   string `help:"Path to config file" short:"c" type:"path"`
	Verbose  bool   `help:"Verbose output" short:"v"`
	Quiet    bool   `help:"Suppress non-essential output" short:"q"`
}

type CLI struct {
	Globals

	Config  ConfigCmd  `cmd:"" help:"Configuration management"`
	Mail    MailCmd    `cmd:"" help:"Email operations"`
	Mailbox MailboxCmd `cmd:"" help:"Mailbox management"`
	Version VersionCmd `cmd:"" help:"Show version information"`
}

type Context struct {
	Config    *config.Config
	Formatter *output.Formatter
	Globals   *Globals
}

func NewContext(globals *Globals) (*Context, error) {
	formatter := output.New(globals.JSON, globals.Verbose, globals.Quiet)

	var cfg *config.Config
	var err error

	if globals.Config != "" {
		cfg, err = config.Load(globals.Config)
	} else if config.Exists() {
		cfg, err = config.Load("")
	}

	if err != nil && cfg == nil {
		cfg = config.DefaultConfig()
	}

	return &Context{
		Config:    cfg,
		Formatter: formatter,
		Globals:   globals,
	}, nil
}

// ConfigCmd handles configuration management
type ConfigCmd struct {
	Init ConfigInitCmd `cmd:"" help:"Interactive setup wizard"`
	Show ConfigShowCmd `cmd:"" help:"Display current configuration"`
	Set  ConfigSetCmd  `cmd:"" help:"Set a configuration value"`
}

type ConfigInitCmd struct{}

type ConfigShowCmd struct{}

type ConfigSetCmd struct {
	Key   string `arg:"" help:"Configuration key (e.g., bridge.email, defaults.limit)"`
	Value string `arg:"" help:"Value to set"`
}

// MailCmd handles email operations
type MailCmd struct {
	List   MailListCmd   `cmd:"" help:"List messages in mailbox"`
	Read   MailReadCmd   `cmd:"" help:"Read a specific message"`
	Send   MailSendCmd   `cmd:"" help:"Compose and send email"`
	Delete MailDeleteCmd `cmd:"" help:"Delete message(s)"`
	Move   MailMoveCmd   `cmd:"" help:"Move message to mailbox"`
	Flag   MailFlagCmd   `cmd:"" help:"Manage message flags"`
	Search MailSearchCmd `cmd:"" help:"Search messages"`
}

type MailListCmd struct {
	Mailbox string `help:"Mailbox name" short:"m" default:"INBOX"`
	Limit   int    `help:"Number of messages" short:"n" default:"20"`
	Unread  bool   `help:"Only show unread messages"`
}

type MailReadCmd struct {
	ID      string `arg:"" help:"Message ID or sequence number"`
	Raw     bool   `help:"Show raw message"`
	Headers bool   `help:"Include all headers"`
}

type MailSendCmd struct {
	To      []string `help:"Recipient(s)" short:"t" required:""`
	CC      []string `help:"CC recipients"`
	BCC     []string `help:"BCC recipients"`
	Subject string   `help:"Subject line" short:"s" required:""`
	Body    string   `help:"Body text (or use stdin)" short:"b"`
	Attach  []string `help:"Attachments" short:"a" type:"existingfile"`
}

type MailDeleteCmd struct {
	IDs       []string `arg:"" help:"Message ID(s) to delete"`
	Permanent bool     `help:"Skip trash, delete permanently"`
}

type MailMoveCmd struct {
	ID      string `arg:"" help:"Message ID to move"`
	Mailbox string `arg:"" help:"Destination mailbox"`
}

type MailFlagCmd struct {
	ID     string `arg:"" help:"Message ID"`
	Read   bool   `help:"Mark as read" xor:"read"`
	Unread bool   `help:"Mark as unread" xor:"read"`
	Star   bool   `help:"Add star" xor:"star"`
	Unstar bool   `help:"Remove star" xor:"star"`
}

type MailSearchCmd struct {
	Query   string `arg:"" help:"Search query"`
	Mailbox string `help:"Mailbox to search" short:"m" default:"INBOX"`
	From    string `help:"Filter by sender"`
	Subject string `help:"Filter by subject"`
	Since   string `help:"Messages since date (YYYY-MM-DD)"`
	Before  string `help:"Messages before date (YYYY-MM-DD)"`
}

// MailboxCmd handles mailbox management
type MailboxCmd struct {
	List   MailboxListCmd   `cmd:"" help:"List all mailboxes/folders"`
	Create MailboxCreateCmd `cmd:"" help:"Create new mailbox"`
	Delete MailboxDeleteCmd `cmd:"" help:"Delete mailbox"`
}

type MailboxListCmd struct{}

type MailboxCreateCmd struct {
	Name string `arg:"" help:"Mailbox name to create"`
}

type MailboxDeleteCmd struct {
	Name string `arg:"" help:"Mailbox name to delete"`
}

// VersionCmd shows version information
type VersionCmd struct{}
