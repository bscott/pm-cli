package cli

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"zero bytes", 0, "0 B"},
		{"bytes", 500, "500 B"},
		{"exactly 1KB", 1024, "1.0 KB"},
		{"kilobytes", 2048, "2.0 KB"},
		{"megabytes", 1048576, "1.0 MB"},
		{"large megabytes", 5242880, "5.0 MB"},
		{"gigabytes", 1073741824, "1.0 GB"},
		{"mixed size", 1536, "1.5 KB"},
		{"small file", 100, "100 B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSize(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestExtractEmailAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple email", "user@example.com", "user@example.com"},
		{"with name", "John Doe <john@example.com>", "john@example.com"},
		{"name with quotes", "\"John Doe\" <john@example.com>", "john@example.com"},
		{"only angle brackets", "<user@example.com>", "user@example.com"},
		{"with spaces", "  user@example.com  ", "user@example.com"},
		{"complex name", "John \"Johnny\" Doe <john@example.com>", "john@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractEmailAddress(tt.input)
			if result != tt.expected {
				t.Errorf("extractEmailAddress(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHtmlToText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains []string
		excludes []string
	}{
		{
			name:     "simple paragraph",
			input:    "<p>Hello World</p>",
			contains: []string{"Hello World"},
			excludes: []string{"<p>", "</p>"},
		},
		{
			name:     "with links",
			input:    `<a href="https://example.com">Click here</a>`,
			contains: []string{"Click here", "https://example.com"},
			excludes: []string{"<a", "</a>"},
		},
		{
			name:     "with br tags",
			input:    "Line 1<br>Line 2<br/>Line 3",
			contains: []string{"Line 1", "Line 2", "Line 3"},
			excludes: []string{"<br>", "<br/>"},
		},
		{
			name:     "removes style blocks",
			input:    "<style>.class { color: red; }</style><p>Content</p>",
			contains: []string{"Content"},
			excludes: []string{"<style>", "color", "red"},
		},
		{
			name:     "removes script blocks",
			input:    "<script>alert('test');</script><p>Content</p>",
			contains: []string{"Content"},
			excludes: []string{"<script>", "alert"},
		},
		{
			name:     "decodes entities",
			input:    "<p>Hello &amp; World &lt;test&gt;</p>",
			contains: []string{"Hello & World <test>"},
			excludes: []string{"&amp;", "&lt;", "&gt;"},
		},
		{
			name:     "handles div blocks",
			input:    "<div>Block 1</div><div>Block 2</div>",
			contains: []string{"Block 1", "Block 2"},
			excludes: []string{"<div>", "</div>"},
		},
		{
			name:     "handles headings",
			input:    "<h1>Title</h1><p>Content</p>",
			contains: []string{"Title", "Content"},
			excludes: []string{"<h1>", "</h1>"},
		},
		{
			name:     "handles lists",
			input:    "<ul><li>Item 1</li><li>Item 2</li></ul>",
			contains: []string{"Item 1", "Item 2"},
			excludes: []string{"<ul>", "<li>", "</li>"},
		},
		{
			name:     "empty input",
			input:    "",
			contains: []string{},
			excludes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := htmlToText(tt.input)

			for _, s := range tt.contains {
				if !containsStr(result, s) {
					t.Errorf("htmlToText() result should contain %q, got %q", s, result)
				}
			}

			for _, s := range tt.excludes {
				if containsStr(result, s) {
					t.Errorf("htmlToText() result should not contain %q, got %q", s, result)
				}
			}
		})
	}
}

func TestParseMessageBody(t *testing.T) {
	tests := []struct {
		name         string
		rawBody      []byte
		wantText     bool
		wantHTML     bool
		textContains string
		htmlContains string
	}{
		{
			name:         "plain text body",
			rawBody:      []byte("Content-Type: text/plain\r\n\r\nHello, World!"),
			wantText:     true,
			wantHTML:     false,
			textContains: "Hello, World!",
		},
		{
			name:         "html body",
			rawBody:      []byte("Content-Type: text/html\r\n\r\n<p>Hello, World!</p>"),
			wantText:     false,
			wantHTML:     true,
			htmlContains: "<p>Hello, World!</p>",
		},
		{
			name:     "empty body",
			rawBody:  []byte(""),
			wantText: true, // Falls back to treating as plain text
			wantHTML: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textBody, htmlBody := parseMessageBody(tt.rawBody)

			if tt.wantText && tt.textContains != "" {
				if !containsStr(textBody, tt.textContains) {
					t.Errorf("textBody should contain %q, got %q", tt.textContains, textBody)
				}
			}

			if tt.wantHTML && tt.htmlContains != "" {
				if !containsStr(htmlBody, tt.htmlContains) {
					t.Errorf("htmlBody should contain %q, got %q", tt.htmlContains, htmlBody)
				}
			}
		})
	}
}

func TestFormatAttributes(t *testing.T) {
	tests := []struct {
		name     string
		attrs    []string
		expected string
	}{
		{"empty", []string{}, ""},
		{"single attribute", []string{"\\HasChildren"}, "HasChildren"},
		{"multiple attributes", []string{"\\HasChildren", "\\Drafts"}, "HasChildren, Drafts"},
		{"no backslash", []string{"Custom"}, "Custom"},
		{"mixed", []string{"\\System", "Custom"}, "System, Custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAttributes(tt.attrs)
			if result != tt.expected {
				t.Errorf("formatAttributes(%v) = %q, want %q", tt.attrs, result, tt.expected)
			}
		})
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if len(substr) == 0 {
			return true
		}
		if i+len(substr) <= len(s) && s[i:i+len(substr)] == substr {
			return true
		}
	}
	return len(substr) == 0
}

func TestMailListCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailListCmd{
		Mailbox: "INBOX",
		Limit:   20,
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailReadCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailReadCmd{
		ID: "1",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailSendCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailSendCmd{
		To:      []string{"recipient@example.com"},
		Subject: "Test",
		Body:    "Test body",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailSendCmdRunWithoutBody(t *testing.T) {
	cmd := &MailSendCmd{
		To:      []string{"recipient@example.com"},
		Subject: "Test",
		Body:    "", // No body
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "sender@example.com"

	// This will fail because there's no body and no stdin
	// The error should mention no body provided
	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when no body provided")
	}
}

func TestMailDeleteCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailDeleteCmd{
		IDs: []string{"1"},
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailMoveCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailMoveCmd{
		IDs:         []string{"1"},
		Destination: "Archive",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailFlagCmdRunWithoutFlags(t *testing.T) {
	cmd := &MailFlagCmd{
		IDs:    []string{"1"},
		Read:   false,
		Unread: false,
		Star:   false,
		Unstar: false,
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "test@example.com"

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when no flags specified")
	}
}

func TestMailFlagCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailFlagCmd{
		IDs:  []string{"1"},
		Read: true,
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailSearchCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailSearchCmd{
		Query:   "test",
		Mailbox: "INBOX",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailReplyCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailReplyCmd{
		ID:   "1",
		Body: "Reply",
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailForwardCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailForwardCmd{
		ID: "1",
		To: []string{"forward@example.com"},
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}

func TestMailDownloadCmdRunWithoutConfig(t *testing.T) {
	cmd := &MailDownloadCmd{
		ID:    "1",
		Index: 0,
	}

	globals := &Globals{}
	ctx, _ := NewContext(globals)
	ctx.Config.Bridge.Email = "" // No email configured

	err := cmd.Run(ctx)
	if err == nil {
		t.Error("expected error when email not configured")
	}
}
