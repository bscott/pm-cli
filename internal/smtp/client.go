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
	"strings"
	"time"

	"github.com/bscott/pm-cli/internal/config"
)

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
	addr := fmt.Sprintf("%s:%d", c.config.Bridge.SMTPHost, c.config.Bridge.SMTPPort)

	// Connect to SMTP server
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	// Wrap with TLS (Proton Bridge uses self-signed cert)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         c.config.Bridge.SMTPHost,
	}
	tlsConn := tls.Client(conn, tlsConfig)

	client, err := smtp.NewClient(tlsConn, c.config.Bridge.SMTPHost)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Close()

	// Authenticate
	auth := smtp.PlainAuth("", c.config.Bridge.Email, c.password, c.config.Bridge.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set sender
	if err := client.Mail(msg.From); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	allRecipients := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	allRecipients = append(allRecipients, msg.To...)
	allRecipients = append(allRecipients, msg.CC...)
	allRecipients = append(allRecipients, msg.BCC...)

	for _, rcpt := range allRecipients {
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

	// Headers
	fmt.Fprintf(w, "From: %s\r\n", msg.From)
	fmt.Fprintf(w, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.CC) > 0 {
		fmt.Fprintf(w, "Cc: %s\r\n", strings.Join(msg.CC, ", "))
	}
	fmt.Fprintf(w, "Subject: %s\r\n", encodeSubject(msg.Subject))
	fmt.Fprintf(w, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	if msg.InReplyTo != "" {
		fmt.Fprintf(w, "In-Reply-To: %s\r\n", msg.InReplyTo)
	}
	if msg.References != "" {
		fmt.Fprintf(w, "References: %s\r\n", msg.References)
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
