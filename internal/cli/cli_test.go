package cli

import (
	"testing"

	"github.com/bscott/pm-cli/internal/config"
)

func TestVersion(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
}

func TestGlobalsStruct(t *testing.T) {
	globals := Globals{
		JSON:     true,
		HelpJSON: true,
		Config:   "/path/to/config.yaml",
		Verbose:  true,
		Quiet:    false,
	}

	if !globals.JSON {
		t.Error("JSON should be true")
	}
	if !globals.HelpJSON {
		t.Error("HelpJSON should be true")
	}
	if globals.Config != "/path/to/config.yaml" {
		t.Errorf("Config = %q, want %q", globals.Config, "/path/to/config.yaml")
	}
	if !globals.Verbose {
		t.Error("Verbose should be true")
	}
	if globals.Quiet {
		t.Error("Quiet should be false")
	}
}

func TestNewContext(t *testing.T) {
	globals := &Globals{
		JSON:    true,
		Verbose: true,
		Quiet:   false,
	}

	ctx, err := NewContext(globals)
	if err != nil {
		t.Fatalf("NewContext() error = %v", err)
	}

	if ctx == nil {
		t.Fatal("expected non-nil context")
	}

	if ctx.Formatter == nil {
		t.Error("Formatter should not be nil")
	}

	if !ctx.Formatter.JSON {
		t.Error("Formatter.JSON should be true")
	}

	if !ctx.Formatter.Verbose {
		t.Error("Formatter.Verbose should be true")
	}

	if ctx.Globals != globals {
		t.Error("Globals not set correctly")
	}
}

func TestNewContextWithConfigPath(t *testing.T) {
	globals := &Globals{
		Config: "/nonexistent/config.yaml",
	}

	ctx, err := NewContext(globals)
	// Should not error even with invalid config path; falls back to defaults
	if err != nil {
		t.Fatalf("NewContext() error = %v", err)
	}

	// Should have a default config
	if ctx.Config == nil {
		t.Error("Config should not be nil (should fall back to defaults)")
	}
}

func TestContextStruct(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config = cfg

	if ctx.Config.Bridge.Email != "test@example.com" {
		t.Errorf("Email = %q, want %q", ctx.Config.Bridge.Email, "test@example.com")
	}
}

func TestMailListCmdDefaults(t *testing.T) {
	cmd := MailListCmd{
		Mailbox: "INBOX",
		Limit:   20,
		Unread:  false,
	}

	if cmd.Mailbox != "INBOX" {
		t.Errorf("Mailbox = %q, want %q", cmd.Mailbox, "INBOX")
	}
	if cmd.Limit != 20 {
		t.Errorf("Limit = %d, want %d", cmd.Limit, 20)
	}
	if cmd.Unread {
		t.Error("Unread should be false by default")
	}
}

func TestMailReadCmdOptions(t *testing.T) {
	cmd := MailReadCmd{
		ID:          "123",
		Raw:         true,
		Headers:     true,
		Attachments: true,
	}

	if cmd.ID != "123" {
		t.Errorf("ID = %q, want %q", cmd.ID, "123")
	}
	if !cmd.Raw {
		t.Error("Raw should be true")
	}
	if !cmd.Headers {
		t.Error("Headers should be true")
	}
	if !cmd.Attachments {
		t.Error("Attachments should be true")
	}
}

func TestMailSendCmdOptions(t *testing.T) {
	cmd := MailSendCmd{
		To:      []string{"recipient@example.com"},
		CC:      []string{"cc@example.com"},
		BCC:     []string{"bcc@example.com"},
		Subject: "Test Subject",
		Body:    "Test Body",
		Attach:  []string{"/path/to/file.pdf"},
	}

	if len(cmd.To) != 1 || cmd.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", cmd.To)
	}
	if len(cmd.CC) != 1 || cmd.CC[0] != "cc@example.com" {
		t.Errorf("CC = %v, want [cc@example.com]", cmd.CC)
	}
	if cmd.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", cmd.Subject, "Test Subject")
	}
}

func TestMailDeleteCmdOptions(t *testing.T) {
	cmd := MailDeleteCmd{
		IDs:       []string{"1", "2", "3"},
		Permanent: true,
	}

	if len(cmd.IDs) != 3 {
		t.Errorf("IDs length = %d, want 3", len(cmd.IDs))
	}
	if !cmd.Permanent {
		t.Error("Permanent should be true")
	}
}

func TestMailMoveCmdOptions(t *testing.T) {
	cmd := MailMoveCmd{
		IDs:         []string{"123"},
		Destination: "Archive",
	}

	if len(cmd.IDs) != 1 || cmd.IDs[0] != "123" {
		t.Errorf("IDs = %v, want [123]", cmd.IDs)
	}
	if cmd.Destination != "Archive" {
		t.Errorf("Destination = %q, want %q", cmd.Destination, "Archive")
	}
}

func TestMailFlagCmdOptions(t *testing.T) {
	cmd := MailFlagCmd{
		IDs:    []string{"123"},
		Read:   true,
		Unread: false,
		Star:   true,
		Unstar: false,
	}

	if len(cmd.IDs) != 1 || cmd.IDs[0] != "123" {
		t.Errorf("IDs = %v, want [123]", cmd.IDs)
	}
	if !cmd.Read {
		t.Error("Read should be true")
	}
	if cmd.Unread {
		t.Error("Unread should be false")
	}
	if !cmd.Star {
		t.Error("Star should be true")
	}
}

func TestMailSearchCmdOptions(t *testing.T) {
	cmd := MailSearchCmd{
		Query:   "test query",
		Mailbox: "INBOX",
		From:    "sender@example.com",
		Subject: "important",
		Since:   "2024-01-01",
		Before:  "2024-12-31",
	}

	if cmd.Query != "test query" {
		t.Errorf("Query = %q, want %q", cmd.Query, "test query")
	}
	if cmd.Mailbox != "INBOX" {
		t.Errorf("Mailbox = %q, want %q", cmd.Mailbox, "INBOX")
	}
	if cmd.From != "sender@example.com" {
		t.Errorf("From = %q, want %q", cmd.From, "sender@example.com")
	}
}

func TestMailReplyCmdOptions(t *testing.T) {
	cmd := MailReplyCmd{
		ID:     "123",
		All:    true,
		Body:   "My reply",
		Attach: []string{"/path/to/file.pdf"},
	}

	if cmd.ID != "123" {
		t.Errorf("ID = %q, want %q", cmd.ID, "123")
	}
	if !cmd.All {
		t.Error("All should be true")
	}
	if cmd.Body != "My reply" {
		t.Errorf("Body = %q, want %q", cmd.Body, "My reply")
	}
}

func TestMailForwardCmdOptions(t *testing.T) {
	cmd := MailForwardCmd{
		ID:     "123",
		To:     []string{"forward@example.com"},
		Body:   "FYI",
		Attach: []string{"/path/to/file.pdf"},
	}

	if cmd.ID != "123" {
		t.Errorf("ID = %q, want %q", cmd.ID, "123")
	}
	if len(cmd.To) != 1 || cmd.To[0] != "forward@example.com" {
		t.Errorf("To = %v, want [forward@example.com]", cmd.To)
	}
}

func TestMailDownloadCmdOptions(t *testing.T) {
	cmd := MailDownloadCmd{
		ID:    "123",
		Index: 0,
		Out:   "/path/to/output.pdf",
	}

	if cmd.ID != "123" {
		t.Errorf("ID = %q, want %q", cmd.ID, "123")
	}
	if cmd.Index != 0 {
		t.Errorf("Index = %d, want 0", cmd.Index)
	}
	if cmd.Out != "/path/to/output.pdf" {
		t.Errorf("Out = %q, want %q", cmd.Out, "/path/to/output.pdf")
	}
}

func TestMailboxCreateCmdOptions(t *testing.T) {
	cmd := MailboxCreateCmd{
		Name: "NewFolder",
	}

	if cmd.Name != "NewFolder" {
		t.Errorf("Name = %q, want %q", cmd.Name, "NewFolder")
	}
}

func TestMailboxDeleteCmdOptions(t *testing.T) {
	cmd := MailboxDeleteCmd{
		Name: "OldFolder",
	}

	if cmd.Name != "OldFolder" {
		t.Errorf("Name = %q, want %q", cmd.Name, "OldFolder")
	}
}

func TestConfigSetCmdOptions(t *testing.T) {
	cmd := ConfigSetCmd{
		Key:   "bridge.email",
		Value: "user@protonmail.com",
	}

	if cmd.Key != "bridge.email" {
		t.Errorf("Key = %q, want %q", cmd.Key, "bridge.email")
	}
	if cmd.Value != "user@protonmail.com" {
		t.Errorf("Value = %q, want %q", cmd.Value, "user@protonmail.com")
	}
}
