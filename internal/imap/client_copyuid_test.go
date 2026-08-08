package imap

import (
	"net"
	"testing"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

// The no-match detection in CopyMessages/MoveMessages reads the COPYUID
// response code, which only servers advertising UIDPLUS (or IMAP4rev2, which
// folds it in) are required to send. On a server without that capability an
// empty COPYUID is simply absent data, so concluding "nothing was copied" would
// turn every successful copy into an error.
//
// client_affected_test.go pins its fixture to IMAP4rev2, so it cannot catch
// that regression. This fixture runs plain IMAP4rev1 instead.

// newLegacyIMAPFixture starts an in-memory server that advertises only
// IMAP4rev1 (no UIDPLUS), seeds one message in INBOX, and returns a bound
// Client plus the seeded UID.
func newLegacyIMAPFixture(t *testing.T) (*Client, imap.UID) {
	t.Helper()

	memServer := imapmemserver.New()
	memServer.AddUser(imapmemserver.NewUser(testUser, testPass))

	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}},
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

	t.Cleanup(func() {
		_ = raw.Close()
		_ = srv.Close()
		_ = ln.Close()
	})

	return &Client{client: raw, config: cfg}, uid
}

// TestCopyWithoutUIDPlusStillSucceeds is the regression guard: a real copy must
// not be reported as "no messages matched" just because the server withheld
// COPYUID.
func TestCopyWithoutUIDPlusStillSucceeds(t *testing.T) {
	client, uid := newLegacyIMAPFixture(t)

	if client.client.Caps().Has(imap.CapUIDPlus) {
		t.Skip("fixture unexpectedly advertises UIDPLUS; nothing to assert")
	}

	if err := client.CopyMessages("INBOX", []string{presentSelector(uid)}, "Archive"); err != nil {
		t.Fatalf("CopyMessages on a non-UIDPLUS server should succeed, got %v", err)
	}
}

// TestMoveWithoutUIDPlusStillSucceeds covers the same path in MoveMessages,
// where the STORE affected-count still provides real no-match detection.
func TestMoveWithoutUIDPlusStillSucceeds(t *testing.T) {
	client, uid := newLegacyIMAPFixture(t)

	if err := client.MoveMessages("INBOX", []string{presentSelector(uid)}, "Archive"); err != nil {
		t.Fatalf("MoveMessages on a non-UIDPLUS server should succeed, got %v", err)
	}
}

// TestMoveMissingUIDWithoutUIDPlusStillErrors confirms the fix does not blunt
// the actual bug fix: with COPYUID unavailable, the STORE affected-count must
// still catch a move of UIDs that are not present.
func TestMoveMissingUIDWithoutUIDPlusStillErrors(t *testing.T) {
	client, uid := newLegacyIMAPFixture(t)

	if err := client.MoveMessages("INBOX", []string{missingSelector(uid)}, "Archive"); err == nil {
		t.Fatal("MoveMessages on a non-existent UID should still error without UIDPLUS")
	}
}

// TestCopyMatchedNothingNilData guards the nil branch of the helper.
func TestCopyMatchedNothingNilData(t *testing.T) {
	client, _ := newLegacyIMAPFixture(t)

	if client.copyMatchedNothing(nil) {
		t.Error("nil CopyData must not be read as a no-match")
	}
}
