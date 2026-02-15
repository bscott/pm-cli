package smtp

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bscott/pm-cli/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"

	client := NewClient(cfg, "testpassword")

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.config != cfg {
		t.Error("config not set correctly")
	}
	if client.password != "testpassword" {
		t.Error("password not set correctly")
	}
}

func TestMessageStruct(t *testing.T) {
	msg := Message{
		From:        "sender@example.com",
		To:          []string{"recipient@example.com"},
		CC:          []string{"cc@example.com"},
		BCC:         []string{"bcc@example.com"},
		Subject:     "Test Subject",
		Body:        "Test Body",
		Attachments: []string{"/path/to/file.pdf"},
		InReplyTo:   "<original-id@example.com>",
		References:  "<original-id@example.com>",
	}

	if msg.From != "sender@example.com" {
		t.Errorf("From = %q, want %q", msg.From, "sender@example.com")
	}
	if len(msg.To) != 1 || msg.To[0] != "recipient@example.com" {
		t.Errorf("To = %v, want [recipient@example.com]", msg.To)
	}
	if len(msg.CC) != 1 || msg.CC[0] != "cc@example.com" {
		t.Errorf("CC = %v, want [cc@example.com]", msg.CC)
	}
	if len(msg.BCC) != 1 || msg.BCC[0] != "bcc@example.com" {
		t.Errorf("BCC = %v, want [bcc@example.com]", msg.BCC)
	}
	if msg.Subject != "Test Subject" {
		t.Errorf("Subject = %q, want %q", msg.Subject, "Test Subject")
	}
	if msg.Body != "Test Body" {
		t.Errorf("Body = %q, want %q", msg.Body, "Test Body")
	}
	if msg.InReplyTo != "<original-id@example.com>" {
		t.Errorf("InReplyTo = %q, want %q", msg.InReplyTo, "<original-id@example.com>")
	}
	if msg.References != "<original-id@example.com>" {
		t.Errorf("References = %q, want %q", msg.References, "<original-id@example.com>")
	}
}

func TestEncodeSubject(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		needsEnc bool
	}{
		{"ascii only", "Hello World", false},
		{"with special chars", "Test: Important!", false},
		{"unicode chars", "Hello \u00e9\u00e0\u00fc", true},
		{"japanese", "\u3053\u3093\u306b\u3061\u306f", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodeSubject(tt.input)

			if tt.needsEnc {
				// Should be RFC 2047 encoded (starts with =?utf-8?)
				if !strings.HasPrefix(result, "=?utf-8?") {
					t.Errorf("encodeSubject(%q) = %q, expected RFC 2047 encoding", tt.input, result)
				}
			} else {
				// Should remain unchanged
				if result != tt.input {
					t.Errorf("encodeSubject(%q) = %q, want %q", tt.input, result, tt.input)
				}
			}
		})
	}
}

func TestWriteMessageSimple(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "sender@example.com"

	client := NewClient(cfg, "testpassword")

	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test Subject",
		Body:    "This is the message body.",
	}

	var buf bytes.Buffer
	err := client.writeMessage(&buf, msg)
	if err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	output := buf.String()

	// Verify headers
	if !strings.Contains(output, "From: sender@example.com") {
		t.Error("output should contain From header")
	}
	if !strings.Contains(output, "To: recipient@example.com") {
		t.Error("output should contain To header")
	}
	if !strings.Contains(output, "Subject: Test Subject") {
		t.Error("output should contain Subject header")
	}
	if !strings.Contains(output, "MIME-Version: 1.0") {
		t.Error("output should contain MIME-Version header")
	}
	if !strings.Contains(output, "Content-Type: text/plain; charset=utf-8") {
		t.Error("output should contain Content-Type header for plain text")
	}
	if !strings.Contains(output, "This is the message body.") {
		t.Error("output should contain message body")
	}
}

func TestWriteMessageWithCC(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewClient(cfg, "testpassword")

	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		CC:      []string{"cc1@example.com", "cc2@example.com"},
		Subject: "Test",
		Body:    "Body",
	}

	var buf bytes.Buffer
	err := client.writeMessage(&buf, msg)
	if err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Cc: cc1@example.com, cc2@example.com") {
		t.Error("output should contain Cc header with both addresses")
	}
}

func TestWriteMessageWithReplyHeaders(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewClient(cfg, "testpassword")

	msg := &Message{
		From:       "sender@example.com",
		To:         []string{"recipient@example.com"},
		Subject:    "Re: Original Subject",
		Body:       "My reply",
		InReplyTo:  "<original-id@example.com>",
		References: "<original-id@example.com>",
	}

	var buf bytes.Buffer
	err := client.writeMessage(&buf, msg)
	if err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "In-Reply-To: <original-id@example.com>") {
		t.Error("output should contain In-Reply-To header")
	}
	if !strings.Contains(output, "References: <original-id@example.com>") {
		t.Error("output should contain References header")
	}
}

func TestWriteMessageWithAttachment(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewClient(cfg, "testpassword")

	// Create a temporary file to attach
	tmpDir, err := os.MkdirTemp("", "smtp-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	attachPath := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(attachPath, []byte("attachment content"), 0644)
	if err != nil {
		t.Fatalf("failed to create attachment: %v", err)
	}

	msg := &Message{
		From:        "sender@example.com",
		To:          []string{"recipient@example.com"},
		Subject:     "Test with attachment",
		Body:        "See attached",
		Attachments: []string{attachPath},
	}

	var buf bytes.Buffer
	err = client.writeMessage(&buf, msg)
	if err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	output := buf.String()

	// Should be multipart message
	if !strings.Contains(output, "Content-Type: multipart/mixed") {
		t.Error("output should be multipart/mixed for attachments")
	}
	if !strings.Contains(output, "Content-Disposition: attachment") {
		t.Error("output should contain attachment disposition")
	}
	if !strings.Contains(output, `filename="test.txt"`) {
		t.Error("output should contain attachment filename")
	}
}

func TestWriteMessageAttachmentNotFound(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewClient(cfg, "testpassword")

	msg := &Message{
		From:        "sender@example.com",
		To:          []string{"recipient@example.com"},
		Subject:     "Test",
		Body:        "Body",
		Attachments: []string{"/nonexistent/file.pdf"},
	}

	var buf bytes.Buffer
	err := client.writeMessage(&buf, msg)
	if err == nil {
		t.Error("expected error for non-existent attachment")
	}
}

func TestWriteMessageMultipleRecipients(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewClient(cfg, "testpassword")

	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"recipient1@example.com", "recipient2@example.com"},
		Subject: "Test",
		Body:    "Body",
	}

	var buf bytes.Buffer
	err := client.writeMessage(&buf, msg)
	if err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "To: recipient1@example.com, recipient2@example.com") {
		t.Error("output should contain both recipients in To header")
	}
}

func TestWriteMessageDateHeader(t *testing.T) {
	cfg := config.DefaultConfig()
	client := NewClient(cfg, "testpassword")

	msg := &Message{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test",
		Body:    "Body",
	}

	var buf bytes.Buffer
	err := client.writeMessage(&buf, msg)
	if err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Date:") {
		t.Error("output should contain Date header")
	}
}

func TestEncodeSubjectSpecialCases(t *testing.T) {
	// Test edge cases
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},                 // Empty string
		{"ASCII", "ASCII"},       // Pure ASCII
		{"a", "a"},               // Single char
		{"123", "123"},           // Numbers only
		{"!@#$%", "!@#$%"},       // ASCII special chars
	}

	for _, tt := range tests {
		result := encodeSubject(tt.input)
		if result != tt.expected {
			t.Errorf("encodeSubject(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestClientConfigValues(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.SMTPHost = "smtp.example.com"
	cfg.Bridge.SMTPPort = 587

	client := NewClient(cfg, "password123")

	if client.config.Bridge.SMTPHost != "smtp.example.com" {
		t.Errorf("SMTPHost = %q, want %q", client.config.Bridge.SMTPHost, "smtp.example.com")
	}
	if client.config.Bridge.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want %d", client.config.Bridge.SMTPPort, 587)
	}
}
