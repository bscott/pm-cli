package smtp

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/bscott/pm-cli/internal/safetext"
)

// isLoopbackHost reports whether host is a loopback address. Accepts the
// literal "localhost" (case-insensitive) or any IP that parses as loopback.
// Does not resolve DNS, because DNS is itself untrusted and we want a hard
// guarantee that we are speaking to a local process (Proton Bridge).
func isLoopbackHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return false
	}
	if h == "localhost" {
		return true
	}
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

type Client struct {
	config   *config.Config
	password string
}

type Message struct {
	From        string
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Body        string
	Attachments []string
	InReplyTo   string
	References  string
}

func NewClient(cfg *config.Config, password string) *Client {
	return &Client{
		config:   cfg,
		password: password,
	}
}

func (c *Client) Send(msg *Message) error {
	// TLS skip-verify below is only safe against a locally-running Proton
	// Bridge. Refuse to speak plaintext credentials to anything that is not
	// a loopback address, in case the user's config was tampered with or
	// redirected via --config.
	if !isLoopbackHost(c.config.Bridge.SMTPHost) {
		return fmt.Errorf("refusing to connect: SMTP host %q is not a loopback address (Proton Bridge runs on localhost; InsecureSkipVerify is unsafe for remote hosts)", c.config.Bridge.SMTPHost)
	}

	addr := net.JoinHostPort(c.config.Bridge.SMTPHost, strconv.Itoa(c.config.Bridge.SMTPPort))

	// Connect to SMTP server using STARTTLS
	// Proton Bridge SMTP uses STARTTLS (connect plain, then upgrade)
	client, err := DialClient(addr, c.config.Bridge.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()

	// Upgrade to TLS via STARTTLS (Proton Bridge uses self-signed cert)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         c.config.Bridge.SMTPHost,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	// Authenticate
	auth := smtp.PlainAuth("", c.config.Bridge.Email, c.password, c.config.Bridge.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender. Reject CR/LF in envelope addresses — net/smtp writes these
	// raw into MAIL FROM / RCPT TO lines.
	if strings.ContainsAny(msg.From, "\r\n") {
		return fmt.Errorf("invalid sender address: contains CR/LF")
	}
	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	allRecipients := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	allRecipients = append(allRecipients, msg.To...)
	allRecipients = append(allRecipients, msg.CC...)
	allRecipients = append(allRecipients, msg.BCC...)

	for _, rcpt := range allRecipients {
		if strings.ContainsAny(rcpt, "\r\n") {
			return fmt.Errorf("invalid recipient address: contains CR/LF")
		}
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to add recipient %s: %w", rcpt, err)
		}
	}

	// Build message
	data, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to start data: %w", err)
	}

	if err := c.writeMessage(data, msg); err != nil {
		data.Close()
		return fmt.Errorf("failed to write message: %w", err)
	}

	if err := data.Close(); err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	return client.Quit()
}

func (c *Client) writeMessage(w io.Writer, msg *Message) error {
	hasAttachments := len(msg.Attachments) > 0

	// Headers — sanitize every value for CR/LF. Subject/InReplyTo/References
	// can carry attacker-controlled data (e.g., the Message-ID of a received
	// email copied into In-Reply-To on reply); without sanitization an
	// attacker can inject additional headers or body content.
	fmt.Fprintf(w, "From: %s\r\n", sanitizeAddressList([]string{msg.From}))
	fmt.Fprintf(w, "To: %s\r\n", sanitizeAddressList(msg.To))
	if len(msg.CC) > 0 {
		fmt.Fprintf(w, "Cc: %s\r\n", sanitizeAddressList(msg.CC))
	}
	fmt.Fprintf(w, "Subject: %s\r\n", encodeSubject(safetext.SanitizeHeaderValue(msg.Subject)))
	fmt.Fprintf(w, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	if msg.InReplyTo != "" {
		fmt.Fprintf(w, "In-Reply-To: %s\r\n", safetext.SanitizeHeaderValue(msg.InReplyTo))
	}
	if msg.References != "" {
		fmt.Fprintf(w, "References: %s\r\n", safetext.SanitizeHeaderValue(msg.References))
	}
	fmt.Fprintf(w, "MIME-Version: 1.0\r\n")

	if !hasAttachments {
		fmt.Fprintf(w, "Content-Type: text/plain; charset=utf-8\r\n")
		fmt.Fprintf(w, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(w, "\r\n")
		fmt.Fprintf(w, "%s\r\n", msg.Body)
		return nil
	}

	// Multipart message with attachments
	var buf bytes.Buffer
	mpWriter := multipart.NewWriter(&buf)

	fmt.Fprintf(w, "Content-Type: multipart/mixed; boundary=%s\r\n", mpWriter.Boundary())
	fmt.Fprintf(w, "\r\n")

	// Text body part
	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Content-Transfer-Encoding", "quoted-printable")

	part, err := mpWriter.CreatePart(header)
	if err != nil {
		return err
	}
	part.Write([]byte(msg.Body))

	// Attachment parts
	for _, attachPath := range msg.Attachments {
		file, err := os.Open(attachPath)
		if err != nil {
			return fmt.Errorf("failed to open attachment %s: %w", attachPath, err)
		}

		content, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("failed to read attachment %s: %w", attachPath, err)
		}

		filename := filepath.Base(attachPath)
		contentType := mime.TypeByExtension(filepath.Ext(filename))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		header := make(textproto.MIMEHeader)
		header.Set("Content-Type", contentType)
		header.Set("Content-Transfer-Encoding", "base64")
		header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

		part, err := mpWriter.CreatePart(header)
		if err != nil {
			return err
		}

		encoded := base64.StdEncoding.EncodeToString(content)
		// Write base64 in chunks of 76 characters
		for len(encoded) > 76 {
			part.Write([]byte(encoded[:76] + "\r\n"))
			encoded = encoded[76:]
		}
		if len(encoded) > 0 {
			part.Write([]byte(encoded))
		}
	}

	mpWriter.Close()
	w.Write(buf.Bytes())

	return nil
}

// sanitizeAddressList strips CR/LF from each address and joins with ", ".
// Addresses are ordinarily CLI-supplied (not attacker-controlled) but the
// same CRLF sanitizer still applies to prevent injection if one is ever
// derived from email content (e.g. a reply-to address).
func sanitizeAddressList(addrs []string) string {
	clean := make([]string, len(addrs))
	for i, a := range addrs {
		clean[i] = safetext.SanitizeHeaderValue(a)
	}
	return strings.Join(clean, ", ")
}

func encodeSubject(subject string) string {
	// Check if encoding is needed (non-ASCII characters)
	needsEncoding := false
	for _, r := range subject {
		if r > 127 {
			needsEncoding = true
			break
		}
	}

	if !needsEncoding {
		return subject
	}

	// Use RFC 2047 encoding
	return mime.QEncoding.Encode("utf-8", subject)
}
