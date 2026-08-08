package smtp

import (
	"bytes"
	"io"
	"mime/quotedprintable"
	"os"
	"strings"
	"testing"
)

// decodeBodyAfterHeaders returns the message body decoded as quoted-printable,
// which is what a conforming receiver does given the declared
// Content-Transfer-Encoding.
func decodeBodyAfterHeaders(t *testing.T, raw string) string {
	t.Helper()
	idx := strings.Index(raw, "\r\n\r\n")
	if idx < 0 {
		t.Fatalf("no header/body separator in message:\n%s", raw)
	}
	body := raw[idx+4:]

	decoded, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	if err != nil {
		t.Fatalf("body is not valid quoted-printable: %v", err)
	}
	return string(decoded)
}

func writeSimple(t *testing.T, body string) string {
	t.Helper()
	var buf bytes.Buffer
	c := &Client{}
	msg := &Message{From: "me@x.test", To: []string{"you@x.test"}, Subject: "s", Body: body}
	if err := c.writeMessage(&buf, msg); err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}
	return buf.String()
}

// TestBodyRoundTripsThroughQuotedPrintable is the regression guard: the
// message declared quoted-printable while writing the raw body, so any "="
// was misdecoded by receivers.
func TestBodyRoundTripsThroughQuotedPrintable(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"plain ascii", "Hello there."},
		{"equals sign", "a = b and c=d"},
		{"formula", "SUM(A1:A2)=42"},
		{"non-ascii", "Café — naïve résumé"},
		{"trailing space", "line with trailing space   \nnext"},
		{"long line", strings.Repeat("abcdefghij", 30)},
		{"multiline", "first\nsecond\nthird"},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := writeSimple(t, tc.body)

			if !strings.Contains(raw, "Content-Transfer-Encoding: quoted-printable") {
				t.Fatal("message does not declare quoted-printable")
			}

			got := decodeBodyAfterHeaders(t, raw)
			// quoted-printable is a line-oriented encoding; normalize the CRLF
			// the encoder emits back to the LF the input used.
			got = strings.ReplaceAll(got, "\r\n", "\n")
			if got != tc.body {
				t.Errorf("round trip changed the body:\n got %q\nwant %q", got, tc.body)
			}
		})
	}
}

// TestEqualsSignIsEncoded pins the specific corruption: a literal "=" must not
// appear unencoded in a quoted-printable body.
func TestEqualsSignIsEncoded(t *testing.T) {
	raw := writeSimple(t, "a = b")

	idx := strings.Index(raw, "\r\n\r\n")
	body := raw[idx+4:]

	if strings.Contains(body, "a = b") {
		t.Errorf("the raw body was written verbatim under a quoted-printable header: %q", body)
	}
	if !strings.Contains(body, "=3D") {
		t.Errorf("expected the equals sign to be encoded as =3D, got %q", body)
	}
}

// TestMultipartBodyIsEncoded covers the attachment path, which declares
// quoted-printable for its text part too.
func TestMultipartBodyIsEncoded(t *testing.T) {
	dir := t.TempDir()
	attach := dir + "/note.txt"
	if err := writeTestFile(attach, "attachment contents"); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	c := &Client{}
	msg := &Message{
		From:        "me@x.test",
		To:          []string{"you@x.test"},
		Subject:     "s",
		Body:        "total = 100",
		Attachments: []string{attach},
	}
	if err := c.writeMessage(&buf, msg); err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}

	raw := buf.String()
	if strings.Contains(raw, "total = 100") {
		t.Error("multipart text part wrote the raw body under a quoted-printable header")
	}
	if !strings.Contains(raw, "=3D") {
		t.Error("multipart text part did not encode the equals sign")
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
