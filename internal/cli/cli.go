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

	Config   ConfigCmd   `cmd:"" help:"Configuration management"`
	Mail     MailCmd     `cmd:"" help:"Email operations"`
	Mailbox  MailboxCmd  `cmd:"" help:"Mailbox management"`
	Contacts ContactsCmd `cmd:"" help:"Address book management"`
	Version  VersionCmd  `cmd:"" help:"Show version information"`
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
	Init     ConfigInitCmd     `cmd:"" help:"Interactive setup wizard"`
	Show     ConfigShowCmd     `cmd:"" help:"Display current configuration"`
	Set      ConfigSetCmd      `cmd:"" help:"Set a configuration value"`
	Validate ConfigValidateCmd `cmd:"" help:"Test Bridge connection"`
	Doctor   ConfigDoctorCmd   `cmd:"" help:"Diagnose configuration issues"`
}

type ConfigInitCmd struct{}

type ConfigShowCmd struct{}

type ConfigSetCmd struct {
	Key   string `arg:"" help:"Configuration key (e.g., bridge.email, defaults.limit)"`
	Value string `arg:"" help:"Value to set"`
}

type ConfigValidateCmd struct{}

type ConfigDoctorCmd struct{}

// MailCmd handles email operations
type MailCmd struct {
	List     MailListCmd     `cmd:"" help:"List messages in mailbox"`
	Read     MailReadCmd     `cmd:"" help:"Read a specific message"`
	Send     MailSendCmd     `cmd:"" help:"Compose and send email"`
	Reply    MailReplyCmd    `cmd:"" help:"Reply to a message"`
	Forward  MailForwardCmd  `cmd:"" help:"Forward a message"`
	Delete   MailDeleteCmd   `cmd:"" help:"Delete message(s)"`
	Move     MailMoveCmd     `cmd:"" help:"Move message to mailbox"`
	Flag     MailFlagCmd     `cmd:"" help:"Manage message flags"`
	Search   MailSearchCmd   `cmd:"" help:"Search messages"`
	Download MailDownloadCmd `cmd:"" help:"Download attachment"`
	Draft    DraftCmd        `cmd:"" help:"Manage drafts"`
	Thread   MailThreadCmd   `cmd:"" help:"Show conversation thread"`
	Watch    MailWatchCmd    `cmd:"" help:"Watch for new messages"`
	Label    LabelCmd        `cmd:"" help:"Manage message labels"`
	Summarize MailSummarizeCmd `cmd:"" help:"Summarize message for AI processing"`
	Extract   MailExtractCmd   `cmd:"" help:"Extract structured data from message"`
}

type MailSummarizeCmd struct {
	ID      string `arg:"" help:"Message ID to summarize"`
	Mailbox string `help:"Mailbox name" short:"m" default:"INBOX"`
}

type MailExtractCmd struct {
	ID      string `arg:"" help:"Message ID to extract data from"`
	Mailbox string `help:"Mailbox name" short:"m" default:"INBOX"`
}

// LabelCmd handles label management
type LabelCmd struct {
	List   LabelListCmd   `cmd:"" help:"List available labels"`
	Add    LabelAddCmd    `cmd:"" help:"Add label to message(s)"`
	Remove LabelRemoveCmd `cmd:"" help:"Remove label from message(s)"`
}

type LabelListCmd struct{}

type LabelAddCmd struct {
	IDs     []string `arg:"" help:"Message ID(s) to label"`
	Label   string   `help:"Label name to add" short:"l" required:""`
	Mailbox string   `help:"Source mailbox" short:"m" default:"INBOX"`
}

type LabelRemoveCmd struct {
	IDs   []string `arg:"" help:"Message ID(s) to unlabel"`
	Label string   `help:"Label name to remove" short:"l" required:""`
}

type MailWatchCmd struct {
	Mailbox  string `help:"Mailbox to watch" short:"m" default:"INBOX"`
	Interval int    `help:"Poll interval in seconds" short:"i" default:"30"`
	Unread   bool   `help:"Only notify for unread messages" default:"true"`
	Exec     string `help:"Command to execute on new mail (use {} for message ID)" short:"e"`
	Once     bool   `help:"Exit after first new message"`
}

type MailThreadCmd struct {
	ID      string `arg:"" help:"Message ID to show thread for"`
	Mailbox string `help:"Mailbox to search" short:"m" default:"INBOX"`
}

// DraftCmd handles draft management
type DraftCmd struct {
	List   DraftListCmd   `cmd:"" help:"List all drafts"`
	Create DraftCreateCmd `cmd:"" help:"Create a new draft"`
	Edit   DraftEditCmd   `cmd:"" help:"Edit an existing draft"`
	Delete DraftDeleteCmd `cmd:"" help:"Delete a draft"`
}

type DraftListCmd struct {
	Limit int `help:"Number of drafts" short:"n" default:"20"`
}

type DraftCreateCmd struct {
	To      []string `help:"Recipient(s)" short:"t"`
	CC      []string `help:"CC recipients"`
	Subject string   `help:"Subject line" short:"s"`
	Body    string   `help:"Body text" short:"b"`
	Attach  []string `help:"Attachments" short:"a" type:"existingfile"`
}

type DraftEditCmd struct {
	ID      string   `arg:"" help:"Draft ID to edit"`
	To      []string `help:"Recipient(s)" short:"t"`
	CC      []string `help:"CC recipients"`
	Subject string   `help:"Subject line" short:"s"`
	Body    string   `help:"Body text" short:"b"`
	Attach  []string `help:"Attachments" short:"a" type:"existingfile"`
}

type DraftDeleteCmd struct {
	IDs []string `arg:"" help:"Draft ID(s) to delete"`
}

type MailListCmd struct {
	Mailbox string `help:"Mailbox name" short:"m" default:"INBOX"`
	Limit   int    `help:"Number of messages" short:"n" default:"20"`
	Offset  int    `help:"Skip first N messages" default:"0"`
	Page    int    `help:"Page number (1-based, combines with limit)" short:"p" default:"0"`
	Unread  bool   `help:"Only show unread messages"`
}

type MailReadCmd struct {
	ID          string `arg:"" help:"Message ID or sequence number"`
	Raw         bool   `help:"Show raw message"`
	Headers     bool   `help:"Include all headers"`
	Attachments bool   `help:"List attachments"`
	HTML        bool   `help:"Output HTML body instead of plain text"`
}

type MailSendCmd struct {
	To             []string          `help:"Recipient(s)" short:"t"`
	CC             []string          `help:"CC recipients"`
	BCC            []string          `help:"BCC recipients"`
	Subject        string            `help:"Subject line" short:"s"`
	Body           string            `help:"Body text (or use stdin)" short:"b"`
	Attach         []string          `help:"Attachments" short:"a" type:"existingfile"`
	IdempotencyKey string            `help:"Unique key to prevent duplicate sends" name:"idempotency-key"`
	Template       string            `help:"Template file path" name:"template" type:"existingfile"`
	Vars           map[string]string `help:"Template variables (key=value)" short:"V"`
}

type MailReplyCmd struct {
	ID             string   `arg:"" help:"Message ID to reply to"`
	All            bool     `help:"Reply to all recipients" name:"all"`
	Body           string   `help:"Reply body" short:"b"`
	Attach         []string `help:"Attachments" short:"a" type:"existingfile"`
	IdempotencyKey string   `help:"Unique key to prevent duplicate sends" name:"idempotency-key"`
}

type MailForwardCmd struct {
	ID             string   `arg:"" help:"Message ID to forward"`
	To             []string `help:"Recipient(s)" short:"t" required:""`
	Body           string   `help:"Additional message" short:"b"`
	Attach         []string `help:"Additional attachments" short:"a" type:"existingfile"`
	IdempotencyKey string   `help:"Unique key to prevent duplicate sends" name:"idempotency-key"`
}

type MailDeleteCmd struct {
	IDs       []string `arg:"" optional:"" help:"Message ID(s) to delete"`
	Query     string   `help:"Delete messages matching search query (e.g., 'from:spam@example.com')"`
	Mailbox   string   `help:"Mailbox to operate on" short:"m" default:"INBOX"`
	Permanent bool     `help:"Skip trash, delete permanently"`
}

type MailDownloadCmd struct {
	ID    string `arg:"" help:"Message ID"`
	Index int    `arg:"" help:"Attachment index (0-based)"`
	Out   string `help:"Output path (default: original filename)" short:"o"`
}

type MailMoveCmd struct {
	IDs         []string `arg:"" optional:"" help:"Message ID(s) to move"`
	Destination string   `help:"Destination mailbox" short:"d" required:""`
	Query       string   `help:"Move messages matching search query (e.g., 'subject:newsletter')"`
	Mailbox     string   `help:"Source mailbox" short:"m" default:"INBOX"`
}

type MailFlagCmd struct {
	IDs     []string `arg:"" optional:"" help:"Message ID(s)"`
	Query   string   `help:"Flag messages matching search query (e.g., 'from:user@example.com')"`
	Mailbox string   `help:"Mailbox to operate on" short:"m" default:"INBOX"`
	Read    bool     `help:"Mark as read" xor:"read"`
	Unread  bool     `help:"Mark as unread" xor:"read"`
	Star    bool     `help:"Add star" xor:"star"`
	Unstar  bool     `help:"Remove star" xor:"star"`
}

type MailSearchCmd struct {
	Query          string `arg:"" optional:"" help:"Search query (searches body text)"`
	Mailbox        string `help:"Mailbox to search" short:"m" default:"INBOX"`
	From           string `help:"Filter by sender"`
	To             string `help:"Filter by recipient"`
	Subject        string `help:"Filter by subject"`
	Body           string `help:"Search in message body"`
	Since          string `help:"Messages since date (YYYY-MM-DD)"`
	Before         string `help:"Messages before date (YYYY-MM-DD)"`
	HasAttachments bool   `help:"Only messages with attachments" name:"has-attachments"`
	LargerThan     string `help:"Messages larger than size (e.g., 1M, 500K)" name:"larger-than"`
	SmallerThan    string `help:"Messages smaller than size (e.g., 10M, 1K)" name:"smaller-than"`
	And            bool   `help:"Combine filters with AND (default)" name:"and" xor:"logic" default:"true"`
	Or             bool   `help:"Combine filters with OR" name:"or" xor:"logic"`
	Not            bool   `help:"Negate the search query" name:"not"`
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

// ContactsCmd handles address book management
type ContactsCmd struct {
	List   ContactsListCmd   `cmd:"" help:"List all contacts"`
	Search ContactsSearchCmd `cmd:"" help:"Search contacts"`
	Add    ContactsAddCmd    `cmd:"" help:"Add a contact"`
	Remove ContactsRemoveCmd `cmd:"" help:"Remove a contact"`
}

type ContactsListCmd struct{}

type ContactsSearchCmd struct {
	Query string `arg:"" help:"Search query (matches name or email)"`
}

type ContactsAddCmd struct {
	Email string `arg:"" help:"Contact email address"`
	Name  string `help:"Contact display name" short:"n" name:"name"`
}

type ContactsRemoveCmd struct {
	Email string `arg:"" help:"Contact email address to remove"`
}
