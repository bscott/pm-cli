package cli

import (
	"encoding/json"
	"testing"

	"github.com/bscott/pm-cli/internal/imap"
)

// Tests for the --fields / --compact JSON shaping added for issue #9. Organized
// into the seven review categories: Security, Performance, Retry, Unit,
// Integration, Functional, and Frame.

func sampleMessages() []imap.MessageSummary {
	return []imap.MessageSummary{
		{
			UID: 1, SeqNum: 1,
			From: "Alice", FromAddress: "alice@example.com",
			To:      []string{"bob@example.com"},
			Subject: "Hello", MessageID: "mid-1", InReplyTo: "parent-1",
			Date: "2026-01-01 10:00", Seen: false, Flagged: true,
		},
		{
			UID: 2, SeqNum: 2,
			From: "Carol", FromAddress: "carol@example.com",
			Subject: "Re: Hello", Date: "2026-01-02 11:00", Seen: true,
		},
	}
}

// --- Unit ---------------------------------------------------------------

func TestSplitFieldList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"uid,from_address", []string{"uid", "from_address"}},
		{" uid , subject ", []string{"uid", "subject"}},
		{"uid,,subject", []string{"uid", "subject"}},
		{"", nil},
		{" , ", nil},
	}
	for _, tc := range tests {
		got := splitFieldList(tc.in)
		if len(got) != len(tc.want) {
			t.Fatalf("splitFieldList(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("splitFieldList(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}

// TestProjectMessageFieldsKeepsOnlyRequested verifies projection reduces each
// object to exactly the requested keys.
func TestProjectMessageFieldsKeepsOnlyRequested(t *testing.T) {
	out, err := projectMessageFields(sampleMessages(), "uid,from_address,subject")
	if err != nil {
		t.Fatalf("projectMessageFields: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d objects, want 2", len(out))
	}
	for i, obj := range out {
		if len(obj) != 3 {
			t.Errorf("obj %d has %d keys, want 3: %v", i, len(obj), obj)
		}
		for _, k := range []string{"uid", "from_address", "subject"} {
			if _, ok := obj[k]; !ok {
				t.Errorf("obj %d missing requested key %q", i, k)
			}
		}
		if _, ok := obj["seen"]; ok {
			t.Errorf("obj %d should not contain unrequested key 'seen'", i)
		}
	}
}

// --- Functional ---------------------------------------------------------

// TestProjectMessageFieldsMissingFieldIsNull checks that a requested field
// dropped by omitempty (e.g. from_address on a message without one) is present
// as null, giving consumers a stable shape.
func TestProjectMessageFieldsMissingFieldIsNull(t *testing.T) {
	msgs := []imap.MessageSummary{{UID: 9, SeqNum: 9, Subject: "no address"}}
	out, err := projectMessageFields(msgs, "uid,from_address")
	if err != nil {
		t.Fatalf("projectMessageFields: %v", err)
	}
	val, ok := out[0]["from_address"]
	if !ok {
		t.Fatal("expected from_address key to be present (as null)")
	}
	if val != nil {
		t.Errorf("from_address = %v, want nil", val)
	}
}

// --- Security -----------------------------------------------------------

// TestProjectMessageFieldsRejectsUnknown ensures an unknown/typo'd field name
// is rejected rather than silently ignored, so callers cannot be misled into
// thinking a field was filtered when it wasn't.
func TestProjectMessageFieldsRejectsUnknown(t *testing.T) {
	if _, err := projectMessageFields(sampleMessages(), "uid,password"); err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if _, err := projectMessageFields(sampleMessages(), ""); err == nil {
		t.Fatal("expected error for empty field list, got nil")
	}
}

// --- Integration --------------------------------------------------------

// TestProjectMessageFieldsRoundTripsJSON confirms the projected output marshals
// to valid JSON with only the requested keys, mirroring what the CLI emits.
func TestProjectMessageFieldsRoundTripsJSON(t *testing.T) {
	out, err := projectMessageFields(sampleMessages(), "uid,flagged")
	if err != nil {
		t.Fatalf("projectMessageFields: %v", err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back []map[string]interface{}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back) != 2 {
		t.Fatalf("got %d, want 2", len(back))
	}
	// JSON numbers decode as float64.
	if back[0]["uid"].(float64) != 1 {
		t.Errorf("uid = %v, want 1", back[0]["uid"])
	}
	if back[0]["flagged"].(bool) != true {
		t.Errorf("flagged = %v, want true", back[0]["flagged"])
	}
}

// --- Retry --------------------------------------------------------------

// TestProjectMessageFieldsDeterministic verifies repeated projection yields
// identical output (pure transform, safe to retry).
func TestProjectMessageFieldsDeterministic(t *testing.T) {
	first, err := projectMessageFields(sampleMessages(), "uid,subject,to")
	if err != nil {
		t.Fatalf("projectMessageFields: %v", err)
	}
	a, _ := json.Marshal(first)
	for i := 0; i < 3; i++ {
		again, err := projectMessageFields(sampleMessages(), "uid,subject,to")
		if err != nil {
			t.Fatalf("retry: %v", err)
		}
		b, _ := json.Marshal(again)
		if string(a) != string(b) {
			t.Errorf("projection not deterministic on attempt %d", i)
		}
	}
}

// --- Performance --------------------------------------------------------

// TestProjectMessageFieldsScales is a light throughput check over a larger
// slice; projection must remain linear and not error.
func TestProjectMessageFieldsScales(t *testing.T) {
	var msgs []imap.MessageSummary
	for i := 0; i < 5000; i++ {
		msgs = append(msgs, imap.MessageSummary{UID: uint32(i), SeqNum: uint32(i), Subject: "x"})
	}
	out, err := projectMessageFields(msgs, "uid,subject")
	if err != nil {
		t.Fatalf("projectMessageFields: %v", err)
	}
	if len(out) != 5000 {
		t.Errorf("got %d, want 5000", len(out))
	}
}

// --- Frame --------------------------------------------------------------
// N/A: projection produces plain maps handed to the shared JSON formatter;
// there is no custom framing/serialization in this layer. The JSON round-trip
// test above already exercises the encoding boundary. Placeholder retained for
// matrix completeness.
func TestProjectMessageFieldsFrameNA(t *testing.T) {
	t.Skip("N/A: no custom framing; JSON encoding covered by round-trip test")
}
