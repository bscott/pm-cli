package imap

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// Tests for issue #9: server-side filtering (SEARCH UNSEEN / FLAGGED) and the
// additional envelope fields exposed on MessageSummary. Integration tests drive
// a real in-memory IMAP server (go-imap's imapmemserver, no new dependency).
//
// Organized into the seven review categories: Security, Performance, Retry,
// Unit, Integration, Functional, and Frame.

const (
	listTestUser = "user@protonmail.com"
	listTestPass = "bridge-pass"
)

type seedMsg struct {
	from      string // e.g. `Alice <alice@example.com>`
	to        string // e.g. `bob@example.com`
	subject   string
	messageID string
	inReplyTo string
	flags     []imap.Flag
}

// startListFixture boots an in-memory server, appends the given messages to
// INBOX in order, and returns a Client bound to it.
func startListFixture(t *testing.T, msgs []seedMsg) *Client {
	t.Helper()

	memServer := imapmemserver.New()
	user := imapmemserver.NewUser(listTestUser, listTestPass)
	memServer.AddUser(user)

	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev2: {}},
		InsecureAuth: true,
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(ln) //nolint:errcheck

	raw, err := imapclient.DialInsecure(ln.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := raw.Login(listTestUser, listTestPass).Wait(); err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := raw.Create("INBOX", nil).Wait(); err != nil {
		t.Fatalf("create INBOX: %v", err)
	}

	for _, m := range msgs {
		appendSeed(t, raw, "INBOX", m)
	}

	cfg := config.DefaultConfig()
	cfg.Bridge.Email = listTestUser

	t.Cleanup(func() {
		_ = raw.Close()
		_ = srv.Close()
		_ = ln.Close()
	})

	return &Client{client: raw, config: cfg}
}

func appendSeed(t *testing.T, raw *imapclient.Client, mailbox string, m seedMsg) {
	t.Helper()

	var b []byte
	add := func(k, v string) {
		if v != "" {
			b = append(b, fmt.Sprintf("%s: %s\r\n", k, v)...)
		}
	}
	add("From", m.from)
	add("To", m.to)
	add("Subject", m.subject)
	add("Message-ID", m.messageID)
	add("In-Reply-To", m.inReplyTo)
	add("Date", time.Now().Format(time.RFC1123Z))
	b = append(b, "\r\nbody\r\n"...)

	cmd := raw.Append(mailbox, int64(len(b)), &imap.AppendOptions{Flags: m.flags})
	if _, err := cmd.Write(b); err != nil {
		t.Fatalf("append write: %v", err)
	}
	if err := cmd.Close(); err != nil {
		t.Fatalf("append close: %v", err)
	}
	if _, err := cmd.Wait(); err != nil {
		t.Fatalf("append wait: %v", err)
	}
}

// --- Unit ---------------------------------------------------------------

// TestPaginateSeqNums exercises the pure pagination helper that slices the
// newest `limit` sequence numbers (after an offset) from an ascending SEARCH
// result.
func TestPaginateSeqNums(t *testing.T) {
	nums := []uint32{1, 2, 3, 4, 5}
	tests := []struct {
		name   string
		limit  int
		offset int
		want   []uint32
	}{
		{"newest three", 3, 0, []uint32{3, 4, 5}},
		{"offset skips newest", 2, 1, []uint32{3, 4}},
		{"limit exceeds count", 10, 0, []uint32{1, 2, 3, 4, 5}},
		{"offset past end", 3, 5, nil},
		{"offset equals count", 3, 5, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := paginateSeqNums(nums, tc.limit, tc.offset)
			if len(got) != len(tc.want) {
				t.Fatalf("paginateSeqNums(%d,%d) = %v, want %v", tc.limit, tc.offset, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("paginateSeqNums(%d,%d) = %v, want %v", tc.limit, tc.offset, got, tc.want)
				}
			}
		})
	}
}

// --- Functional ---------------------------------------------------------

// TestListEnvelopeFields verifies the newly exposed envelope fields are
// populated from the ENVELOPE (no extra fetch).
func TestListEnvelopeFields(t *testing.T) {
	client := startListFixture(t, []seedMsg{{
		from:      "Alice Example <alice@example.com>",
		to:        "bob@example.com",
		subject:   "Hello",
		messageID: "<msg-1@example.com>",
		inReplyTo: "<parent@example.com>",
	}})

	msgs, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 10})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	m := msgs[0]

	if m.From != "Alice Example" {
		t.Errorf("From = %q, want display name %q", m.From, "Alice Example")
	}
	if m.FromAddress != "alice@example.com" {
		t.Errorf("FromAddress = %q, want %q", m.FromAddress, "alice@example.com")
	}
	if len(m.To) != 1 || m.To[0] != "bob@example.com" {
		t.Errorf("To = %v, want [bob@example.com]", m.To)
	}
	// go-imap normalizes Message-ID / In-Reply-To by stripping the angle
	// brackets (same value the existing `mail read` path exposes).
	if m.MessageID != "msg-1@example.com" {
		t.Errorf("MessageID = %q, want %q", m.MessageID, "msg-1@example.com")
	}
	if m.InReplyTo != "parent@example.com" {
		t.Errorf("InReplyTo = %q, want %q", m.InReplyTo, "parent@example.com")
	}
}

// --- Integration --------------------------------------------------------

// TestListUnreadServerSide is the key regression from the issue: with the
// newest messages already read, a client-side filter over the newest `limit`
// would return nothing; server-side SEARCH UNSEEN must return the unread ones.
func TestListUnreadServerSide(t *testing.T) {
	// Seq 1-3 unread (older), seq 4-6 read (newer).
	var seed []seedMsg
	for i := 1; i <= 3; i++ {
		seed = append(seed, seedMsg{from: fmt.Sprintf("u%d@example.com", i), subject: "unread"})
	}
	for i := 4; i <= 6; i++ {
		seed = append(seed, seedMsg{from: fmt.Sprintf("r%d@example.com", i), subject: "read", flags: []imap.Flag{imap.FlagSeen}})
	}
	client := startListFixture(t, seed)

	msgs, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 3, UnreadOnly: true})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("SEARCH UNSEEN returned %d messages, want 3 (client-side filtering over the newest %d would miss them)", len(msgs), 3)
	}
	for _, m := range msgs {
		if m.Seen {
			t.Errorf("unread listing returned a seen message: %+v", m)
		}
	}
}

// TestListFlaggedServerSide covers the new --flagged filter (SEARCH FLAGGED).
func TestListFlaggedServerSide(t *testing.T) {
	client := startListFixture(t, []seedMsg{
		{from: "a@example.com", subject: "plain"},
		{from: "b@example.com", subject: "starred", flags: []imap.Flag{imap.FlagFlagged}},
		{from: "c@example.com", subject: "plain2"},
		{from: "d@example.com", subject: "starred2", flags: []imap.Flag{imap.FlagFlagged}},
	})

	msgs, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 20, FlaggedOnly: true})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("SEARCH FLAGGED returned %d, want 2", len(msgs))
	}
	for _, m := range msgs {
		if !m.Flagged {
			t.Errorf("flagged listing returned an unflagged message: %+v", m)
		}
	}
}

// --- Security -----------------------------------------------------------

// TestListEmptyMailboxNoError ensures listing an empty mailbox with a filter is
// a clean empty result, not an error or panic (defensive against unexpected
// SEARCH responses).
func TestListEmptyMailboxNoError(t *testing.T) {
	client := startListFixture(t, nil)

	msgs, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 10, UnreadOnly: true})
	if err != nil {
		t.Fatalf("ListMessages on empty mailbox: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages, got %d", len(msgs))
	}
}

// --- Retry --------------------------------------------------------------

// TestListRepeatableResults verifies repeated identical queries are stable
// (SEARCH + FETCH are read-only; no state drift between calls).
func TestListRepeatableResults(t *testing.T) {
	client := startListFixture(t, []seedMsg{
		{from: "a@example.com", subject: "one"},
		{from: "b@example.com", subject: "two", flags: []imap.Flag{imap.FlagFlagged}},
	})

	first, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 20, FlaggedOnly: true})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	for i := 0; i < 3; i++ {
		again, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 20, FlaggedOnly: true})
		if err != nil {
			t.Fatalf("ListMessages retry: %v", err)
		}
		if len(again) != len(first) {
			t.Errorf("retry %d returned %d, want %d", i, len(again), len(first))
		}
	}
}

// --- Performance --------------------------------------------------------

// TestListUnreadRespectsLimit documents the efficiency win: with server-side
// filtering the result count is bounded by the limit rather than fetching a
// wide range and discarding most of it client-side.
func TestListUnreadRespectsLimit(t *testing.T) {
	var seed []seedMsg
	for i := 0; i < 10; i++ {
		seed = append(seed, seedMsg{from: fmt.Sprintf("u%d@example.com", i), subject: "unread"})
	}
	client := startListFixture(t, seed)

	msgs, err := client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 4, UnreadOnly: true})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 4 {
		t.Errorf("limit=4 unread returned %d, want exactly 4", len(msgs))
	}
}

// --- Frame --------------------------------------------------------------
// N/A: MessageSummary is serialized by the CLI layer (encoding/json); this
// package parses the server's already-framed ENVELOPE/SEARCH responses and adds
// no new wire framing. The integration tests above assert those responses are
// decoded into the right fields. Placeholder retained for matrix completeness.
func TestListFrameNA(t *testing.T) {
	t.Skip("N/A: no new wire framing; envelope/search decoding covered by integration tests")
}
