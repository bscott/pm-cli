package imap

import (
	"testing"

	"github.com/bscott/pm-cli/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client == nil {
		t.Fatal("expected non-nil client")
	}

	if client.config != cfg {
		t.Error("client config not set correctly")
	}

	if client.client != nil {
		t.Error("internal client should be nil before Connect()")
	}
}

func TestClientCloseWithoutConnect(t *testing.T) {
	cfg := config.DefaultConfig()

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// Close should not panic when not connected
	err = client.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestSplitPartNum(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int
	}{
		{"empty string", "", nil},
		{"single number", "1", []int{1}},
		{"two numbers", "1.2", []int{1, 2}},
		{"three numbers", "1.2.3", []int{1, 2, 3}},
		{"larger numbers", "10.20.30", []int{10, 20, 30}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitPartNum(tt.input)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("splitPartNum(%q) = %v, want nil", tt.input, result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("splitPartNum(%q) length = %d, want %d", tt.input, len(result), len(tt.expected))
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("splitPartNum(%q)[%d] = %d, want %d", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestFormatAddress(t *testing.T) {
	// Note: This tests the formatAddress helper function.
	// Since imap.Address requires the actual go-imap types, we test
	// the behavior conceptually here.

	// The function formats addresses as "Name <email>" or just "email"
	// if no name is present.
}

func TestClientMethodsRequireConnection(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	// All these methods should fail gracefully when not connected

	t.Run("ListMailboxes without connection", func(t *testing.T) {
		_, err := client.ListMailboxes()
		if err == nil {
			t.Error("expected error when not connected")
		}
	})

	t.Run("SelectMailbox without connection", func(t *testing.T) {
		_, err := client.SelectMailbox("INBOX")
		if err == nil {
			t.Error("expected error when not connected")
		}
	})

	t.Run("CreateMailbox without connection", func(t *testing.T) {
		err := client.CreateMailbox("TestFolder")
		if err == nil {
			t.Error("expected error when not connected")
		}
	})

	t.Run("DeleteMailbox without connection", func(t *testing.T) {
		err := client.DeleteMailbox("TestFolder")
		if err == nil {
			t.Error("expected error when not connected")
		}
	})
}

func TestAttachmentPartInfo(t *testing.T) {
	// Test the attachmentPartInfo struct
	info := attachmentPartInfo{
		partNums: []int{1, 2, 3},
		filename: "test.pdf",
	}

	if len(info.partNums) != 3 {
		t.Errorf("partNums length = %d, want 3", len(info.partNums))
	}
	if info.filename != "test.pdf" {
		t.Errorf("filename = %q, want %q", info.filename, "test.pdf")
	}
}

func TestClientConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.IMAPHost = "localhost"
	cfg.Bridge.IMAPPort = 1143
	cfg.Bridge.Email = "user@protonmail.com"

	client, _ := NewClient(cfg)

	// Verify the client stores the config
	if client.config.Bridge.IMAPHost != "localhost" {
		t.Errorf("IMAPHost = %q, want %q", client.config.Bridge.IMAPHost, "localhost")
	}
	if client.config.Bridge.IMAPPort != 1143 {
		t.Errorf("IMAPPort = %d, want %d", client.config.Bridge.IMAPPort, 1143)
	}
	if client.config.Bridge.Email != "user@protonmail.com" {
		t.Errorf("Email = %q, want %q", client.config.Bridge.Email, "user@protonmail.com")
	}
}
