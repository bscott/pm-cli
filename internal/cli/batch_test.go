package cli

import (
	"strings"
	"testing"
)

func TestValidateBatchOp(t *testing.T) {
	tests := []struct {
		name    string
		op      batchOp
		wantErr string // substring expected in error, "" means no error
	}{
		{
			name: "valid label",
			op:   batchOp{Op: "label", UIDs: []string{"uid:1"}, Label: "Work"},
		},
		{
			name: "valid move",
			op:   batchOp{Op: "move", UIDs: []string{"uid:1"}, To: "Archive"},
		},
		{
			name: "valid flag read",
			op:   batchOp{Op: "flag", UIDs: []string{"uid:1"}, Read: true},
		},
		{
			name: "valid delete",
			op:   batchOp{Op: "delete", UIDs: []string{"1"}},
		},
		{
			name:    "unknown op",
			op:      batchOp{Op: "frobnicate", UIDs: []string{"1"}},
			wantErr: "unknown op",
		},
		{
			name:    "missing uids",
			op:      batchOp{Op: "delete"},
			wantErr: "uids is required",
		},
		{
			name:    "label missing label name",
			op:      batchOp{Op: "label", UIDs: []string{"1"}},
			wantErr: "label is required",
		},
		{
			name:    "move missing destination",
			op:      batchOp{Op: "move", UIDs: []string{"1"}},
			wantErr: "to is required",
		},
		{
			name:    "flag with no flag set",
			op:      batchOp{Op: "flag", UIDs: []string{"1"}},
			wantErr: "at least one of",
		},
		{
			name:    "label name with IMAP wildcard",
			op:      batchOp{Op: "label", UIDs: []string{"1"}, Label: "bad*name"},
			wantErr: "IMAP special characters",
		},
		{
			name:    "move destination with CRLF",
			op:      batchOp{Op: "move", UIDs: []string{"1"}, To: "Archive\r\nEVIL"},
			wantErr: "IMAP special characters",
		},
		{
			name:    "source mailbox with CRLF rejected",
			op:      batchOp{Op: "delete", UIDs: []string{"1"}, Mailbox: "INBOX\r\nEVIL"},
			wantErr: "IMAP special characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBatchOp(0, tt.op)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestValidateMailboxName(t *testing.T) {
	valid := []string{"", "INBOX", "Labels/Work", "Some Folder", "📚 Learning"}
	for _, name := range valid {
		if err := validateMailboxName(name); err != nil {
			t.Errorf("expected %q to be valid, got %v", name, err)
		}
	}

	invalid := []string{"a*b", "a%b", "a{b", "a\rb", "a\nb"}
	for _, name := range invalid {
		if err := validateMailboxName(name); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
}

func TestReadBatchOps(t *testing.T) {
	t.Run("valid array", func(t *testing.T) {
		ops, err := decodeBatchOps([]byte(`[{"op":"delete","uids":["1"]}]`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(ops) != 1 || ops[0].Op != "delete" {
			t.Fatalf("unexpected ops: %+v", ops)
		}
	})

	t.Run("empty array", func(t *testing.T) {
		if _, err := decodeBatchOps([]byte(`[]`)); err == nil {
			t.Error("expected error for empty array")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		if _, err := decodeBatchOps([]byte(`{not json`)); err == nil {
			t.Error("expected error for invalid json")
		}
	})
}
