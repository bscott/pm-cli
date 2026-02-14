package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/bscott/pm-cli/internal/imap"
	"github.com/bscott/pm-cli/internal/smtp"
	"github.com/emersion/go-message/mail"
)

func (c *MailListCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	mailbox := c.Mailbox
	if mailbox == "" {
		mailbox = ctx.Config.Defaults.Mailbox
	}

	limit := c.Limit
	if limit == 0 {
		limit = ctx.Config.Defaults.Limit
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	ctx.Formatter.Verbosef("Fetching messages from %s...", mailbox)

	messages, err := client.ListMessages(mailbox, limit, c.Unread)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"mailbox":  mailbox,
			"count":    len(messages),
			"messages": messages,
		})
	}

	if len(messages) == 0 {
		fmt.Printf("No %smessages in %s\n", func() string {
			if c.Unread {
				return "unread "
			}
			return ""
		}(), mailbox)
		return nil
	}

	fmt.Printf("Messages in %s (%d):\n\n", mailbox, len(messages))

	table := ctx.Formatter.NewTable("ID", "FLAGS", "FROM", "SUBJECT", "DATE")
	for _, msg := range messages {
		flags := ""
		if !msg.Seen {
			flags += "N" // New/Unread
		}
		if msg.Flagged {
			flags += "*" // Starred
		}
		if flags == "" {
			flags = "-"
		}

		subject := msg.Subject
		if len(subject) > 50 {
			subject = subject[:47] + "..."
		}

		from := msg.From
		if len(from) > 25 {
			from = from[:22] + "..."
		}

		table.AddRow(
			fmt.Sprintf("%d", msg.SeqNum),
			flags,
			from,
			subject,
			msg.Date,
		)
	}
	table.Flush()

	return nil
}

func (c *MailReadCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	msg, err := client.GetMessage(ctx.Config.Defaults.Mailbox, c.ID)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		output := map[string]interface{}{
			"uid":        msg.UID,
			"seq_num":    msg.SeqNum,
			"message_id": msg.MessageID,
			"from":       msg.From,
			"to":         msg.To,
			"cc":         msg.CC,
			"subject":    msg.Subject,
			"date":       msg.Date,
			"flags":      msg.Flags,
		}

		// Parse body
		if len(msg.RawBody) > 0 {
			textBody, htmlBody := parseMessageBody(msg.RawBody)
			if textBody != "" {
				output["body"] = textBody
			}
			if htmlBody != "" {
				output["html_body"] = htmlBody
			}
			if c.Raw {
				output["raw"] = string(msg.RawBody)
			}
		}

		return ctx.Formatter.PrintJSON(output)
	}

	// Text output
	if c.Raw {
		fmt.Println(string(msg.RawBody))
		return nil
	}

	fmt.Printf("From:    %s\n", msg.From)
	fmt.Printf("To:      %s\n", strings.Join(msg.To, ", "))
	if len(msg.CC) > 0 {
		fmt.Printf("CC:      %s\n", strings.Join(msg.CC, ", "))
	}
	fmt.Printf("Date:    %s\n", msg.Date)
	fmt.Printf("Subject: %s\n", msg.Subject)

	if c.Headers {
		fmt.Printf("Flags:   %s\n", strings.Join(msg.Flags, ", "))
		if msg.MessageID != "" {
			fmt.Printf("ID:      %s\n", msg.MessageID)
		}
	}

	fmt.Println()
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()

	// Parse and display body
	if len(msg.RawBody) > 0 {
		textBody, htmlBody := parseMessageBody(msg.RawBody)
		if textBody != "" {
			fmt.Println(textBody)
		} else if htmlBody != "" {
			// Convert HTML to plain text
			text := htmlToText(htmlBody)
			if text != "" {
				fmt.Println(text)
			} else {
				fmt.Println("[HTML content - use --raw to view]")
			}
		} else {
			fmt.Println("[No body content]")
		}
	}

	return nil
}

func (c *MailSendCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	body := c.Body
	if body == "" {
		// Read from stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			var lines []string
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			body = strings.Join(lines, "\n")
		}
	}

	if body == "" {
		return fmt.Errorf("no message body provided - use --body or pipe via stdin")
	}

	password, err := ctx.Config.GetPassword()
	if err != nil {
		return err
	}

	smtpClient := smtp.NewClient(ctx.Config, password)

	msg := &smtp.Message{
		From:        ctx.Config.Bridge.Email,
		To:          c.To,
		CC:          c.CC,
		BCC:         c.BCC,
		Subject:     c.Subject,
		Body:        body,
		Attachments: c.Attach,
	}

	ctx.Formatter.Verbosef("Sending email to %s...", strings.Join(c.To, ", "))

	if err := smtpClient.Send(msg); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"message": "Email sent successfully",
			"to":      c.To,
			"subject": c.Subject,
		})
	}

	fmt.Println("Email sent successfully.")
	return nil
}

func (c *MailDeleteCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	if err := client.DeleteMessages(ctx.Config.Defaults.Mailbox, c.IDs, c.Permanent); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":   true,
			"deleted":   c.IDs,
			"permanent": c.Permanent,
		})
	}

	action := "moved to trash"
	if c.Permanent {
		action = "permanently deleted"
	}
	fmt.Printf("Message(s) %s %s.\n", strings.Join(c.IDs, ", "), action)
	return nil
}

func (c *MailMoveCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	if err := client.MoveMessage(ctx.Config.Defaults.Mailbox, c.ID, c.Mailbox); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":     true,
			"message_id":  c.ID,
			"destination": c.Mailbox,
		})
	}

	fmt.Printf("Message %s moved to %s.\n", c.ID, c.Mailbox)
	return nil
}

func (c *MailFlagCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	if !c.Read && !c.Unread && !c.Star && !c.Unstar {
		return fmt.Errorf("no flags specified - use --read, --unread, --star, or --unstar")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	if err := client.SetFlags(ctx.Config.Defaults.Mailbox, c.ID, c.Read, c.Unread, c.Star, c.Unstar); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":    true,
			"message_id": c.ID,
			"read":       c.Read,
			"unread":     c.Unread,
			"star":       c.Star,
			"unstar":     c.Unstar,
		})
	}

	var changes []string
	if c.Read {
		changes = append(changes, "marked as read")
	}
	if c.Unread {
		changes = append(changes, "marked as unread")
	}
	if c.Star {
		changes = append(changes, "starred")
	}
	if c.Unstar {
		changes = append(changes, "unstarred")
	}

	fmt.Printf("Message %s %s.\n", c.ID, strings.Join(changes, ", "))
	return nil
}

func (c *MailSearchCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	messages, err := client.Search(c.Mailbox, c.Query, c.From, c.Subject, c.Since, c.Before)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"query":    c.Query,
			"mailbox":  c.Mailbox,
			"count":    len(messages),
			"messages": messages,
		})
	}

	if len(messages) == 0 {
		fmt.Println("No messages found.")
		return nil
	}

	fmt.Printf("Found %d message(s):\n\n", len(messages))

	table := ctx.Formatter.NewTable("ID", "FLAGS", "FROM", "SUBJECT", "DATE")
	for _, msg := range messages {
		flags := ""
		if !msg.Seen {
			flags += "N"
		}
		if msg.Flagged {
			flags += "*"
		}
		if flags == "" {
			flags = "-"
		}

		subject := msg.Subject
		if len(subject) > 50 {
			subject = subject[:47] + "..."
		}

		from := msg.From
		if len(from) > 25 {
			from = from[:22] + "..."
		}

		table.AddRow(
			fmt.Sprintf("%d", msg.SeqNum),
			flags,
			from,
			subject,
			msg.Date,
		)
	}
	table.Flush()

	return nil
}

func parseMessageBody(rawBody []byte) (textBody, htmlBody string) {
	reader, err := mail.CreateReader(bytes.NewReader(rawBody))
	if err != nil {
		// Fallback: treat as plain text
		return string(rawBody), ""
	}
	defer reader.Close()

	// Check the top-level content type for single-part messages
	header := reader.Header
	contentType := header.Get("Content-Type")

	// Try to iterate through parts (for multipart messages)
	foundParts := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}
		foundParts = true

		partContentType := part.Header.Get("Content-Type")

		switch {
		case strings.HasPrefix(partContentType, "text/plain"):
			body, err := io.ReadAll(part.Body)
			if err == nil {
				textBody = string(body)
			}
		case strings.HasPrefix(partContentType, "text/html"):
			body, err := io.ReadAll(part.Body)
			if err == nil {
				htmlBody = string(body)
			}
		}
	}

	// Handle single-part messages (no parts found)
	if !foundParts {
		// Find body after headers (double newline)
		rawStr := string(rawBody)
		if idx := strings.Index(rawStr, "\r\n\r\n"); idx != -1 {
			body := rawStr[idx+4:]
			if strings.HasPrefix(contentType, "text/html") {
				htmlBody = body
			} else {
				textBody = body
			}
		} else if idx := strings.Index(rawStr, "\n\n"); idx != -1 {
			body := rawStr[idx+2:]
			if strings.HasPrefix(contentType, "text/html") {
				htmlBody = body
			} else {
				textBody = body
			}
		}
	}

	return textBody, htmlBody
}

// htmlToText converts HTML to plain text by stripping tags and decoding entities
func htmlToText(htmlContent string) string {
	// Remove style and script blocks
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	text := reStyle.ReplaceAllString(htmlContent, "")
	text = reScript.ReplaceAllString(text, "")

	// Replace common block elements with newlines
	reBlock := regexp.MustCompile(`(?i)</(p|div|tr|li|h[1-6])>`)
	text = reBlock.ReplaceAllString(text, "\n")

	// Replace <br> with newlines
	reBr := regexp.MustCompile(`(?i)<br\s*/?>`)
	text = reBr.ReplaceAllString(text, "\n")

	// Extract link URLs
	reLink := regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']+)["'][^>]*>([^<]*)</a>`)
	text = reLink.ReplaceAllString(text, "$2 [$1]")

	// Remove all remaining HTML tags
	reTags := regexp.MustCompile(`<[^>]+>`)
	text = reTags.ReplaceAllString(text, "")

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Clean up whitespace
	reSpaces := regexp.MustCompile(`[ \t]+`)
	text = reSpaces.ReplaceAllString(text, " ")

	reNewlines := regexp.MustCompile(`\n{3,}`)
	text = reNewlines.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
