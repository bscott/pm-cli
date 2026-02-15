package imap

import (
	"encoding/json"
	"testing"
)

func TestMailboxInfoJSON(t *testing.T) {
	mb := MailboxInfo{
		Name:       "INBOX",
		Delimiter:  "/",
		Attributes: []string{"\\HasNoChildren"},
	}

	data, err := json.Marshal(mb)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result MailboxInfo
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Name != mb.Name {
		t.Errorf("Name = %q, want %q", result.Name, mb.Name)
	}
	if result.Delimiter != mb.Delimiter {
		t.Errorf("Delimiter = %q, want %q", result.Delimiter, mb.Delimiter)
	}
	if len(result.Attributes) != len(mb.Attributes) {
		t.Errorf("Attributes length = %d, want %d", len(result.Attributes), len(mb.Attributes))
	}
}

func TestMailboxStatusJSON(t *testing.T) {
	status := MailboxStatus{
		Name:     "INBOX",
		Messages: 100,
		Recent:   5,
		Unseen:   10,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result MailboxStatus
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Name != status.Name {
		t.Errorf("Name = %q, want %q", result.Name, status.Name)
	}
	if result.Messages != status.Messages {
		t.Errorf("Messages = %d, want %d", result.Messages, status.Messages)
	}
	if result.Recent != status.Recent {
		t.Errorf("Recent = %d, want %d", result.Recent, status.Recent)
	}
	if result.Unseen != status.Unseen {
		t.Errorf("Unseen = %d, want %d", result.Unseen, status.Unseen)
	}
}

func TestMessageSummaryJSON(t *testing.T) {
	summary := MessageSummary{
		UID:     12345,
		SeqNum:  1,
		From:    "sender@example.com",
		Subject: "Test Subject",
		Date:    "2024-01-15 10:30",
		Seen:    true,
		Flagged: false,
	}

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result MessageSummary
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.UID != summary.UID {
		t.Errorf("UID = %d, want %d", result.UID, summary.UID)
	}
	if result.SeqNum != summary.SeqNum {
		t.Errorf("SeqNum = %d, want %d", result.SeqNum, summary.SeqNum)
	}
	if result.From != summary.From {
		t.Errorf("From = %q, want %q", result.From, summary.From)
	}
	if result.Subject != summary.Subject {
		t.Errorf("Subject = %q, want %q", result.Subject, summary.Subject)
	}
	if result.Date != summary.Date {
		t.Errorf("Date = %q, want %q", result.Date, summary.Date)
	}
	if result.Seen != summary.Seen {
		t.Errorf("Seen = %v, want %v", result.Seen, summary.Seen)
	}
	if result.Flagged != summary.Flagged {
		t.Errorf("Flagged = %v, want %v", result.Flagged, summary.Flagged)
	}
}

func TestMessageJSON(t *testing.T) {
	msg := Message{
		UID:       12345,
		SeqNum:    1,
		MessageID: "<message-id@example.com>",
		From:      "sender@example.com",
		To:        []string{"recipient@example.com"},
		CC:        []string{"cc@example.com"},
		Subject:   "Test Subject",
		Date:      "2024-01-15 10:30:00",
		Flags:     []string{"\\Seen"},
		TextBody:  "Hello, World!",
		HTMLBody:  "<p>Hello, World!</p>",
		RawBody:   []byte("raw content"),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result Message
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.UID != msg.UID {
		t.Errorf("UID = %d, want %d", result.UID, msg.UID)
	}
	if result.MessageID != msg.MessageID {
		t.Errorf("MessageID = %q, want %q", result.MessageID, msg.MessageID)
	}
	if result.From != msg.From {
		t.Errorf("From = %q, want %q", result.From, msg.From)
	}
	if result.Subject != msg.Subject {
		t.Errorf("Subject = %q, want %q", result.Subject, msg.Subject)
	}
	if len(result.To) != len(msg.To) {
		t.Errorf("To length = %d, want %d", len(result.To), len(msg.To))
	}
	if len(result.CC) != len(msg.CC) {
		t.Errorf("CC length = %d, want %d", len(result.CC), len(msg.CC))
	}

	// RawBody should be excluded from JSON (json:"-")
	if len(result.RawBody) != 0 {
		t.Error("RawBody should not be serialized to JSON")
	}
}

func TestAttachmentJSON(t *testing.T) {
	att := Attachment{
		Index:       0,
		Filename:    "document.pdf",
		ContentType: "application/pdf",
		Size:        1024,
		Data:        []byte("binary data"),
	}

	data, err := json.Marshal(att)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var result Attachment
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Index != att.Index {
		t.Errorf("Index = %d, want %d", result.Index, att.Index)
	}
	if result.Filename != att.Filename {
		t.Errorf("Filename = %q, want %q", result.Filename, att.Filename)
	}
	if result.ContentType != att.ContentType {
		t.Errorf("ContentType = %q, want %q", result.ContentType, att.ContentType)
	}
	if result.Size != att.Size {
		t.Errorf("Size = %d, want %d", result.Size, att.Size)
	}

	// Data should be excluded from JSON (json:"-")
	if len(result.Data) != 0 {
		t.Error("Data should not be serialized to JSON")
	}
}

func TestMessageSummaryFlags(t *testing.T) {
	tests := []struct {
		name    string
		seen    bool
		flagged bool
	}{
		{"unread", false, false},
		{"read", true, false},
		{"flagged unread", false, true},
		{"read and flagged", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := MessageSummary{
				Seen:    tt.seen,
				Flagged: tt.flagged,
			}

			if summary.Seen != tt.seen {
				t.Errorf("Seen = %v, want %v", summary.Seen, tt.seen)
			}
			if summary.Flagged != tt.flagged {
				t.Errorf("Flagged = %v, want %v", summary.Flagged, tt.flagged)
			}
		})
	}
}

func TestMailboxInfoAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attributes []string
	}{
		{"no attributes", []string{}},
		{"single attribute", []string{"\\Noselect"}},
		{"multiple attributes", []string{"\\HasChildren", "\\Drafts"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mb := MailboxInfo{
				Name:       "Test",
				Attributes: tt.attributes,
			}

			if len(mb.Attributes) != len(tt.attributes) {
				t.Errorf("Attributes length = %d, want %d", len(mb.Attributes), len(tt.attributes))
			}
		})
	}
}
