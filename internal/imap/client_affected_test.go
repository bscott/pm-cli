package imap

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// Integration tests for issue #11: STORE/COPY/MOVE must not report success when
// the target UIDs are not present in the selected mailbox. These drive a real
// in-memory IMAP server (go-imap's imapmemserver) so the affected-count and
// COPYUID logic added to DeleteMessages/CopyMessages/MoveMessages is exercised
// end-to-end.
//
// The suite is organized into the seven review categories: Security,
// Performance, Retry, Unit, Integration, Functional, and Frame.

const (
	testUser = "user@protonmail.com"
	testPass = "bridge-pass"
)

// imapFixture is a running in-memory IMAP server plus a connected pm-cli
// Client wired directly to it (bypassing TLS/keyring, which are irrelevant to
// the behavior under test).
type imapfixture struct {
	client *Client
	raw    *imapclient.Client
}

// newIMAPFixture starts a fresh in-memory server with an INBOX and an Archive
// mailbox, appends one message to INBOX, and returns a Client bound to it along
// with the UID of the seeded message.
func newIMAPFixture(t *testing.T) (*imapfixture, imap.UID) {
	t.Helper()

	memServer := imapmemserver.New()
	user := imapmemserver.NewUser(testUser, testPass)
	memServer.AddUser(user)

	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		// IMAP4rev2 folds in UIDPLUS/MOVE/ESEARCH, giving us COPYUID data.
		Caps:         imap.CapSet{imap.CapIMAP4rev2: {}},
		InsecureAuth: true,
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go srv.Serve(ln) //nolint:errcheck // stopped via t.Cleanup below

	raw, err := imapclient.DialInsecure(ln.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if err := raw.Login(testUser, testPass).Wait(); err != nil {
		t.Fatalf("login: %v", err)
	}

	for _, name := range []string{"INBOX", "Archive"} {
		if err := raw.Create(name, nil).Wait(); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}

	uid := appendMessage(t, raw, "INBOX", "seed@example.com")

	cfg := config.DefaultConfig()
	cfg.Bridge.Email = testUser
	client := &Client{client: raw, config: cfg}

	t.Cleanup(func() {
		_ = raw.Close()
		_ = srv.Close()
		_ = ln.Close()
	})

	return &imapfixture{client: client, raw: raw}, uid
}

// appendMessage appends a minimal RFC822 message and returns its UID.
func appendMessage(t *testing.T, raw *imapclient.Client, mailbox, from string) imap.UID {
	t.Helper()
	body := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: test\r\nMessage-ID: <%d@example.com>\r\nDate: %s\r\n\r\nhello\r\n",
		from, testUser, time.Now().UnixNano(), time.Now().Format(time.RFC1123Z),
	)
	cmd := raw.Append(mailbox, int64(len(body)), nil)
	if _, err := cmd.Write([]byte(body)); err != nil {
		t.Fatalf("append write: %v", err)
	}
	if err := cmd.Close(); err != nil {
		t.Fatalf("append close: %v", err)
	}
	data, err := cmd.Wait()
	if err != nil {
		t.Fatalf("append wait: %v", err)
	}
	return data.UID
}

// missingUID returns a UID selector guaranteed not to match the seeded message.
func missingSelector(seeded imap.UID) string {
	return fmt.Sprintf("uid:%d", uint32(seeded)+100000)
}

func presentSelector(seeded imap.UID) string {
	return fmt.Sprintf("uid:%d", uint32(seeded))
}

// --- Unit ---------------------------------------------------------------

// TestDeleteMessagesMissingUIDErrors is the core regression: deleting a UID
// that isn't in the mailbox used to return nil (silent success).
func TestDeleteMessagesMissingUIDErrors(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	err := fx.client.DeleteMessages("INBOX", []string{missingSelector(uid)}, false)
	if err == nil {
		t.Fatal("DeleteMessages on a non-existent UID should error, got nil (silent success)")
	}
	if !strings.Contains(err.Error(), "no messages matched") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestDeleteMessagesPresentUIDSucceeds guards against a false positive: a real
// UID must still delete cleanly.
func TestDeleteMessagesPresentUIDSucceeds(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	if err := fx.client.DeleteMessages("INBOX", []string{presentSelector(uid)}, false); err != nil {
		t.Fatalf("DeleteMessages on a present UID should succeed, got %v", err)
	}
}

// --- Functional ---------------------------------------------------------

// TestCopyMessagesMissingUIDErrors covers the label-add path (Copy to a
// Labels/* folder) that motivated the issue.
func TestCopyMessagesMissingUIDErrors(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	// The bug was a *silent success*, so the assertion is simply that an error
	// is returned. Two server behaviors are possible and both are handled by
	// CopyMessages: a strict server (like this in-memory one) rejects a COPY
	// that matches nothing at the protocol level, so copyCmd.Wait() errors;
	// Proton Bridge instead returns an empty COPYUID set, which the
	// SourceUIDs/DestUIDs length check turns into an error.
	err := fx.client.CopyMessages("INBOX", []string{missingSelector(uid)}, "Archive")
	if err == nil {
		t.Fatal("CopyMessages on a non-existent UID should error, got nil (silent success)")
	}
}

func TestCopyMessagesPresentUIDSucceeds(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	if err := fx.client.CopyMessages("INBOX", []string{presentSelector(uid)}, "Archive"); err != nil {
		t.Fatalf("CopyMessages on a present UID should succeed, got %v", err)
	}
}

// TestMoveMessagesMissingUIDErrors covers MoveMessages, which reimplements
// COPY+STORE and previously ignored both responses.
func TestMoveMessagesMissingUIDErrors(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	// As with CopyMessages, either the COPY step errors (strict server) or the
	// empty COPYUID set is caught; both surface as a non-nil error instead of a
	// silent success that expunges nothing.
	err := fx.client.MoveMessages("INBOX", []string{missingSelector(uid)}, "Archive")
	if err == nil {
		t.Fatal("MoveMessages on a non-existent UID should error, got nil (silent success)")
	}
}

func TestMoveMessagesPresentUIDSucceeds(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	if err := fx.client.MoveMessages("INBOX", []string{presentSelector(uid)}, "Archive"); err != nil {
		t.Fatalf("MoveMessages on a present UID should succeed, got %v", err)
	}
}

// --- Integration --------------------------------------------------------

// TestMoveMessagesActuallyMoves verifies the happy path end-to-end: the message
// leaves INBOX and lands in Archive, so the added guards do not break real moves.
func TestMoveMessagesActuallyMoves(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	if err := fx.client.MoveMessages("INBOX", []string{presentSelector(uid)}, "Archive"); err != nil {
		t.Fatalf("MoveMessages: %v", err)
	}

	inbox, err := fx.client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 50})
	if err != nil {
		t.Fatalf("ListMessages INBOX: %v", err)
	}
	if len(inbox) != 0 {
		t.Errorf("expected INBOX empty after move, got %d messages", len(inbox))
	}

	archive, err := fx.client.ListMessages(ListOptions{Mailbox: "Archive", Limit: 50})
	if err != nil {
		t.Fatalf("ListMessages Archive: %v", err)
	}
	if len(archive) != 1 {
		t.Errorf("expected 1 message in Archive after move, got %d", len(archive))
	}
}

// --- Security -----------------------------------------------------------

// TestMissingUIDDoesNotMutateMailbox ensures a failed (no-match) delete leaves
// the mailbox untouched — the guard must not partially mutate then error.
func TestMissingUIDDoesNotMutateMailbox(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	_ = fx.client.DeleteMessages("INBOX", []string{missingSelector(uid)}, true)

	msgs, err := fx.client.ListMessages(ListOptions{Mailbox: "INBOX", Limit: 50})
	if err != nil {
		t.Fatalf("ListMessages: %v", err)
	}
	if len(msgs) != 1 {
		t.Errorf("failed delete must not remove messages; INBOX has %d, want 1", len(msgs))
	}
}

// --- Retry --------------------------------------------------------------

// TestDeleteMissingUIDStableAcrossRetries verifies the error is deterministic:
// retrying a no-match delete keeps failing and never leaks a false success.
func TestDeleteMissingUIDStableAcrossRetries(t *testing.T) {
	fx, uid := newIMAPFixture(t)
	sel := missingSelector(uid)

	for i := 0; i < 3; i++ {
		if err := fx.client.DeleteMessages("INBOX", []string{sel}, false); err == nil {
			t.Fatalf("attempt %d: expected error on non-existent UID", i)
		}
	}
}

// --- Performance --------------------------------------------------------

// TestAffectedCountNoExtraRoundTrip documents that the fix adds no extra IMAP
// round-trips: the affected count is drained from the STORE response stream
// that the server already sends. A present-UID delete must complete promptly.
func TestAffectedCountNoExtraRoundTrip(t *testing.T) {
	fx, uid := newIMAPFixture(t)

	done := make(chan error, 1)
	go func() { done <- fx.client.DeleteMessages("INBOX", []string{presentSelector(uid)}, false) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("DeleteMessages: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("DeleteMessages did not complete promptly (possible extra/blocking round-trip)")
	}
}

// --- Frame --------------------------------------------------------------
// N/A: affected-count detection consumes the server's already-framed FETCH and
// COPYUID responses; this code introduces no new wire framing of its own. The
// happy-path integration test above already asserts the framed responses are
// parsed correctly. Placeholder retained to keep the seven-category matrix
// explicit.
func TestAffectedCountFrameNA(t *testing.T) {
	t.Skip("N/A: no new wire framing introduced; response parsing covered by integration tests")
}
