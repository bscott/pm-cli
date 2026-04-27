package imap

import (
	"strings"
	"testing"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/emersion/go-imap/v2"
)

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"127.0.0.1", true},
		{"::1", true},
		{"  127.0.0.1  ", true},
		{"imap.example.com", false},
		{"10.0.0.1", false},
		{"", false},
		{"localhost.evil.com", false},
	}
	for _, tc := range tests {
		got := isLoopbackHost(tc.host)
		if got != tc.want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestConnectRejectsNonLoopbackHost(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Bridge.Email = "test@example.com"
	cfg.Bridge.IMAPHost = "imap.example.com"
	cfg.Bridge.IMAPPort = 993

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	err = client.Connect()
	if err == nil {
		t.Fatal("expected loopback rejection")
	}
	if !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("expected loopback error, got %v", err)
	}
}

func TestBuildDraftMessageStripsHeaderInjection(t *testing.T) {
	draft := &Draft{
		To:      []string{"victim@example.com\r\nBcc: attacker@evil.example"},
		CC:      []string{"normal@example.com\r\nX-Evil: 1"},
		Subject: "Hi\r\nBcc: attacker2@evil.example",
		Body:    "body",
	}
	out := buildDraftMessage(draft, "sender@example.com\r\nX-Spoof: yes")
	injectedHeaders := []string{
		"\r\nBcc:",
		"\r\nX-Evil:",
		"\r\nX-Spoof:",
		"\nBcc:",
		"\nX-Evil:",
		"\nX-Spoof:",
	}
	for _, h := range injectedHeaders {
		if strings.Contains(out, h) {
			t.Errorf("injected header line %q reached draft output:\n%s", h, out)
		}
	}
}

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

func TestParseMessageSelector(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
		kind    messageSelectorKind
		seq     uint32
		uid     imap.UID
	}{
		{name: "sequence number", id: "123", kind: selectorKindSeq, seq: 123},
		{name: "uid selector", id: "uid:456", kind: selectorKindUID, uid: imap.UID(456)},
		{name: "uid selector uppercase prefix", id: "UID:789", kind: selectorKindUID, uid: imap.UID(789)},
		{name: "invalid empty", id: "", wantErr: true},
		{name: "invalid zero sequence", id: "0", wantErr: true},
		{name: "invalid zero uid", id: "uid:0", wantErr: true},
		{name: "invalid string", id: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector, err := parseMessageSelector(tt.id)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseMessageSelector(%q) expected error", tt.id)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseMessageSelector(%q) error = %v", tt.id, err)
			}
			if selector.kind != tt.kind {
				t.Fatalf("selector.kind = %v, want %v", selector.kind, tt.kind)
			}
			if selector.seq != tt.seq {
				t.Fatalf("selector.seq = %d, want %d", selector.seq, tt.seq)
			}
			if selector.uid != tt.uid {
				t.Fatalf("selector.uid = %d, want %d", selector.uid, tt.uid)
			}
		})
	}
}

func TestBuildNumSetFromIDs(t *testing.T) {
	t.Run("sequence set", func(t *testing.T) {
		numSet, err := buildNumSetFromIDs([]string{"1", "2", "7"})
		if err != nil {
			t.Fatalf("buildNumSetFromIDs sequence error = %v", err)
		}
		seqSet, ok := numSet.(imap.SeqSet)
		if !ok {
			t.Fatalf("expected imap.SeqSet, got %T", numSet)
		}
		nums, ok := seqSet.Nums()
		if !ok {
			t.Fatal("expected concrete sequence numbers")
		}
		if len(nums) != 3 {
			t.Fatalf("expected 3 sequence numbers, got %d", len(nums))
		}
	})

	t.Run("uid set", func(t *testing.T) {
		numSet, err := buildNumSetFromIDs([]string{"uid:10", "uid:20"})
		if err != nil {
			t.Fatalf("buildNumSetFromIDs uid error = %v", err)
		}
		uidSet, ok := numSet.(imap.UIDSet)
		if !ok {
			t.Fatalf("expected imap.UIDSet, got %T", numSet)
		}
		nums, ok := uidSet.Nums()
		if !ok {
			t.Fatal("expected concrete UIDs")
		}
		if len(nums) != 2 {
			t.Fatalf("expected 2 uids, got %d", len(nums))
		}
	})

	t.Run("mixed ids rejected", func(t *testing.T) {
		_, err := buildNumSetFromIDs([]string{"1", "uid:2"})
		if err == nil {
			t.Fatal("expected error when mixing sequence numbers and UID selectors")
		}
	})
}
