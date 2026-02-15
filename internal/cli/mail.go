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

	// Handle --attachments flag: list attachments only
	if c.Attachments {
		attachments, err := client.GetAttachments(ctx.Config.Defaults.Mailbox, c.ID)
		if err != nil {
			return err
		}

		if ctx.Formatter.JSON {
			return ctx.Formatter.PrintJSON(map[string]interface{}{
				"message_id":  c.ID,
				"attachments": attachments,
				"count":       len(attachments),
			})
		}

		if len(attachments) == 0 {
			fmt.Println("No attachments found.")
			return nil
		}

		fmt.Printf("Attachments (%d):\n\n", len(attachments))
		table := ctx.Formatter.NewTable("INDEX", "FILENAME", "TYPE", "SIZE")
		for _, att := range attachments {
			table.AddRow(
				fmt.Sprintf("%d", att.Index),
				att.Filename,
				att.ContentType,
				formatSize(att.Size),
			)
		}
		table.Flush()
		return nil
	}

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

func (c *MailReplyCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	// Fetch original message
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

	// Build reply subject
	subject := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}

	// Determine recipients
	var recipients []string
	replyTo := extractEmailAddress(msg.From)
	recipients = append(recipients, replyTo)

	var ccRecipients []string
	if c.All {
		// Add all original To recipients except ourselves
		for _, to := range msg.To {
			addr := extractEmailAddress(to)
			if addr != ctx.Config.Bridge.Email && addr != replyTo {
				recipients = append(recipients, addr)
			}
		}
		// Add original CC recipients
		for _, cc := range msg.CC {
			addr := extractEmailAddress(cc)
			if addr != ctx.Config.Bridge.Email && addr != replyTo {
				ccRecipients = append(ccRecipients, addr)
			}
		}
	}

	// Get the body text from original message
	textBody, htmlBody := parseMessageBody(msg.RawBody)
	originalBody := textBody
	if originalBody == "" && htmlBody != "" {
		originalBody = htmlToText(htmlBody)
	}

	// Build quoted body
	var quotedLines []string
	for _, line := range strings.Split(originalBody, "\n") {
		quotedLines = append(quotedLines, "> "+line)
	}
	quotedBody := strings.Join(quotedLines, "\n")

	// Construct full body with reply text
	body := c.Body
	if body == "" {
		// Read from stdin if available
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
		return fmt.Errorf("no reply body provided - use --body or pipe via stdin")
	}

	fullBody := body + "\n\nOn " + msg.Date + ", " + msg.From + " wrote:\n" + quotedBody

	// Build references header
	references := msg.MessageID
	if msg.MessageID != "" {
		references = msg.MessageID
	}

	password, err := ctx.Config.GetPassword()
	if err != nil {
		return err
	}

	smtpClient := smtp.NewClient(ctx.Config, password)

	replyMsg := &smtp.Message{
		From:        ctx.Config.Bridge.Email,
		To:          recipients,
		CC:          ccRecipients,
		Subject:     subject,
		Body:        fullBody,
		Attachments: c.Attach,
		InReplyTo:   msg.MessageID,
		References:  references,
	}

	ctx.Formatter.Verbosef("Sending reply to %s...", strings.Join(recipients, ", "))

	if err := smtpClient.Send(replyMsg); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":      true,
			"message":      "Reply sent successfully",
			"to":           recipients,
			"cc":           ccRecipients,
			"subject":      subject,
			"in_reply_to":  msg.MessageID,
			"reply_all":    c.All,
		})
	}

	fmt.Println("Reply sent successfully.")
	return nil
}

func (c *MailForwardCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	// Fetch original message
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

	// Build forward subject
	subject := msg.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "fwd:") {
		subject = "Fwd: " + subject
	}

	// Get the body text from original message
	textBody, htmlBody := parseMessageBody(msg.RawBody)
	originalBody := textBody
	if originalBody == "" && htmlBody != "" {
		originalBody = htmlToText(htmlBody)
	}

	// Build forwarded message body
	forwardHeader := "---------- Forwarded message ----------\n"
	forwardHeader += "From: " + msg.From + "\n"
	forwardHeader += "Date: " + msg.Date + "\n"
	forwardHeader += "Subject: " + msg.Subject + "\n"
	forwardHeader += "To: " + strings.Join(msg.To, ", ") + "\n"
	forwardHeader += "\n"

	// Add user's message if provided
	body := c.Body
	if body == "" {
		// Read from stdin if available
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

	var fullBody string
	if body != "" {
		fullBody = body + "\n\n" + forwardHeader + originalBody
	} else {
		fullBody = forwardHeader + originalBody
	}

	password, err := ctx.Config.GetPassword()
	if err != nil {
		return err
	}

	smtpClient := smtp.NewClient(ctx.Config, password)

	fwdMsg := &smtp.Message{
		From:        ctx.Config.Bridge.Email,
		To:          c.To,
		Subject:     subject,
		Body:        fullBody,
		Attachments: c.Attach,
	}

	ctx.Formatter.Verbosef("Forwarding email to %s...", strings.Join(c.To, ", "))

	if err := smtpClient.Send(fwdMsg); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":          true,
			"message":          "Email forwarded successfully",
			"to":               c.To,
			"subject":          subject,
			"original_from":    msg.From,
			"original_subject": msg.Subject,
		})
	}

	fmt.Println("Email forwarded successfully.")
	return nil
}

// formatSize returns a human-readable size string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// extractEmailAddress extracts the email address from a formatted address string.
// For example, "John Doe <john@example.com>" returns "john@example.com"
func extractEmailAddress(addr string) string {
	if idx := strings.Index(addr, "<"); idx != -1 {
		if endIdx := strings.Index(addr, ">"); endIdx != -1 {
			return addr[idx+1 : endIdx]
		}
	}
	return strings.TrimSpace(addr)
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

func (c *MailDownloadCmd) Run(ctx *Context) error {
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
		return fmt.Errorf("failed to get message: %w", err)
	}

	// Parse attachments from raw body
	attachments := parseAttachments(msg.RawBody)
	if len(attachments) == 0 {
		return fmt.Errorf("no attachments found in message %s", c.ID)
	}

	if c.Index < 0 || c.Index >= len(attachments) {
		return fmt.Errorf("invalid attachment index %d (message has %d attachments)", c.Index, len(attachments))
	}

	attachment := attachments[c.Index]

	// Determine output path
	outPath := c.Out
	if outPath == "" {
		outPath = attachment.Filename
		if outPath == "" {
			outPath = fmt.Sprintf("attachment_%d", c.Index)
		}
	}

	// Write file
	if err := os.WriteFile(outPath, attachment.Data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":      true,
			"filename":     attachment.Filename,
			"content_type": attachment.ContentType,
			"size":         len(attachment.Data),
			"output_path":  outPath,
		})
	}

	fmt.Printf("Saved %s (%d bytes) to %s\n", attachment.Filename, len(attachment.Data), outPath)
	return nil
}

func parseAttachments(rawBody []byte) []imap.Attachment {
	var attachments []imap.Attachment
	reader, err := mail.CreateReader(bytes.NewReader(rawBody))
	if err != nil {
		return nil
	}
	defer reader.Close()

	index := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		contentType := part.Header.Get("Content-Type")
		contentDisposition := part.Header.Get("Content-Disposition")

		// Check if this is an attachment
		if strings.Contains(contentDisposition, "attachment") ||
			(contentDisposition != "" && !strings.HasPrefix(contentType, "text/")) {
			// Extract filename
			filename := ""
			if strings.Contains(contentDisposition, "filename=") {
				re := regexp.MustCompile(`filename="?([^";]+)"?`)
				if matches := re.FindStringSubmatch(contentDisposition); len(matches) > 1 {
					filename = matches[1]
				}
			}

			data, err := io.ReadAll(part.Body)
			if err != nil {
				continue
			}

			attachments = append(attachments, imap.Attachment{
				Index:       index,
				Filename:    filename,
				ContentType: contentType,
				Size:        int64(len(data)),
				Data:        data,
			})
			index++
		}
	}

	return attachments
}
