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

	// Calculate offset from page if specified
	offset := c.Offset
	if c.Page > 0 {
		offset = (c.Page - 1) * limit
	}

	messages, err := client.ListMessages(mailbox, limit, offset, c.Unread)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		result := map[string]interface{}{
			"mailbox":  mailbox,
			"count":    len(messages),
			"messages": messages,
			"offset":   offset,
			"limit":    limit,
		}
		if c.Page > 0 {
			result["page"] = c.Page
		}
		return ctx.Formatter.PrintJSON(result)
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

		if c.HTML {
			// Output HTML body directly
			if htmlBody != "" {
				fmt.Println(htmlBody)
			} else if textBody != "" {
				// No HTML, output text
				fmt.Println(textBody)
			} else {
				fmt.Println("[No body content]")
			}
		} else {
			// Default: output plain text
			if textBody != "" {
				fmt.Println(textBody)
			} else if htmlBody != "" {
				// Convert HTML to plain text
				text := htmlToText(htmlBody)
				if text != "" {
					fmt.Println(text)
				} else {
					fmt.Println("[HTML content - use --html to view]")
				}
			} else {
				fmt.Println("[No body content]")
			}
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

	// Require either IDs or query
	if len(c.IDs) == 0 && c.Query == "" {
		return fmt.Errorf("provide message ID(s) or use --query to match messages")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	mailbox := c.Mailbox
	if mailbox == "" {
		mailbox = ctx.Config.Defaults.Mailbox
	}

	ids := c.IDs

	// If query is provided, search for matching messages
	if c.Query != "" {
		opts := parseQueryToSearchOptions(c.Query)
		searchIDs, err := client.SearchIDs(mailbox, opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		if len(searchIDs) == 0 {
			if ctx.Formatter.JSON {
				return ctx.Formatter.PrintJSON(map[string]interface{}{
					"success": true,
					"deleted": []string{},
					"message": "No messages matched the query",
				})
			}
			fmt.Println("No messages matched the query.")
			return nil
		}
		ids = searchIDs
		ctx.Formatter.Verbosef("Query matched %d message(s)", len(ids))
	}

	if err := client.DeleteMessages(mailbox, ids, c.Permanent); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":   true,
			"deleted":   ids,
			"count":     len(ids),
			"permanent": c.Permanent,
		})
	}

	action := "moved to trash"
	if c.Permanent {
		action = "permanently deleted"
	}
	fmt.Printf("%d message(s) %s.\n", len(ids), action)
	return nil
}

func (c *MailMoveCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	// Require either IDs or query
	if len(c.IDs) == 0 && c.Query == "" {
		return fmt.Errorf("provide message ID(s) or use --query to match messages")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	mailbox := c.Mailbox
	if mailbox == "" {
		mailbox = ctx.Config.Defaults.Mailbox
	}

	ids := c.IDs

	// If query is provided, search for matching messages
	if c.Query != "" {
		opts := parseQueryToSearchOptions(c.Query)
		searchIDs, err := client.SearchIDs(mailbox, opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		if len(searchIDs) == 0 {
			if ctx.Formatter.JSON {
				return ctx.Formatter.PrintJSON(map[string]interface{}{
					"success":     true,
					"moved":       []string{},
					"destination": c.Destination,
					"message":     "No messages matched the query",
				})
			}
			fmt.Println("No messages matched the query.")
			return nil
		}
		ids = searchIDs
		ctx.Formatter.Verbosef("Query matched %d message(s)", len(ids))
	}

	if err := client.MoveMessages(mailbox, ids, c.Destination); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success":     true,
			"moved":       ids,
			"count":       len(ids),
			"destination": c.Destination,
		})
	}

	fmt.Printf("%d message(s) moved to %s.\n", len(ids), c.Destination)
	return nil
}

func (c *MailFlagCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	if !c.Read && !c.Unread && !c.Star && !c.Unstar {
		return fmt.Errorf("no flags specified - use --read, --unread, --star, or --unstar")
	}

	// Require either IDs or query
	if len(c.IDs) == 0 && c.Query == "" {
		return fmt.Errorf("provide message ID(s) or use --query to match messages")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	mailbox := c.Mailbox
	if mailbox == "" {
		mailbox = ctx.Config.Defaults.Mailbox
	}

	ids := c.IDs

	// If query is provided, search for matching messages
	if c.Query != "" {
		opts := parseQueryToSearchOptions(c.Query)
		searchIDs, err := client.SearchIDs(mailbox, opts)
		if err != nil {
			return fmt.Errorf("search failed: %w", err)
		}
		if len(searchIDs) == 0 {
			if ctx.Formatter.JSON {
				return ctx.Formatter.PrintJSON(map[string]interface{}{
					"success": true,
					"flagged": []string{},
					"message": "No messages matched the query",
				})
			}
			fmt.Println("No messages matched the query.")
			return nil
		}
		ids = searchIDs
		ctx.Formatter.Verbosef("Query matched %d message(s)", len(ids))
	}

	if err := client.SetFlagsMultiple(mailbox, ids, c.Read, c.Unread, c.Star, c.Unstar); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"flagged": ids,
			"count":   len(ids),
			"read":    c.Read,
			"unread":  c.Unread,
			"star":    c.Star,
			"unstar":  c.Unstar,
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

	fmt.Printf("%d message(s) %s.\n", len(ids), strings.Join(changes, ", "))
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

	// Build search options from command flags
	opts := imap.SearchOptions{
		Query:          c.Query,
		From:           c.From,
		To:             c.To,
		Subject:        c.Subject,
		Body:           c.Body,
		Since:          c.Since,
		Before:         c.Before,
		HasAttachments: c.HasAttachments,
		LargerThan:     parseSize(c.LargerThan),
		SmallerThan:    parseSize(c.SmallerThan),
		UseOr:          c.Or,
		Negate:         c.Not,
	}

	messages, err := client.Search(c.Mailbox, opts)
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

// parseQueryString parses a query string like "from:user@example.com subject:test"
// into from, subject, and body components.
// Supports from:, subject:, and body: prefixes. Unprefixed terms search the body.
func parseQueryString(query string) (from, subject, body string) {
	// Simple parser for key:value pairs
	var bodyParts []string

	// Handle quoted strings and key:value pairs
	parts := splitQueryParts(query)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.HasPrefix(strings.ToLower(part), "from:") {
			from = strings.TrimPrefix(part, "from:")
			from = strings.TrimPrefix(from, "FROM:")
			from = strings.Trim(from, "\"")
		} else if strings.HasPrefix(strings.ToLower(part), "subject:") {
			subject = strings.TrimPrefix(part, "subject:")
			subject = strings.TrimPrefix(subject, "SUBJECT:")
			subject = strings.Trim(subject, "\"")
		} else if strings.HasPrefix(strings.ToLower(part), "body:") {
			bodyTerm := strings.TrimPrefix(part, "body:")
			bodyTerm = strings.TrimPrefix(bodyTerm, "BODY:")
			bodyTerm = strings.Trim(bodyTerm, "\"")
			bodyParts = append(bodyParts, bodyTerm)
		} else {
			// Unprefixed terms are body searches
			bodyParts = append(bodyParts, strings.Trim(part, "\""))
		}
	}

	body = strings.Join(bodyParts, " ")
	return from, subject, body
}

// splitQueryParts splits a query string respecting quoted strings.
// "hello world" from:user becomes ["hello world", "from:user"]
func splitQueryParts(query string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(query); i++ {
		c := query[i]
		if c == '"' {
			inQuotes = !inQuotes
			current.WriteByte(c)
		} else if c == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
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

// parseSize parses size strings like "1M", "500K", "1G" into bytes
func parseSize(s string) int64 {
	if s == "" {
		return 0
	}

	s = strings.TrimSpace(strings.ToUpper(s))
	if len(s) == 0 {
		return 0
	}

	multiplier := int64(1)
	suffix := s[len(s)-1]

	switch suffix {
	case 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'G':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case 'B':
		// Handle "KB", "MB", "GB" suffixes
		if len(s) >= 2 {
			prefix := s[len(s)-2]
			switch prefix {
			case 'K':
				multiplier = 1024
				s = s[:len(s)-2]
			case 'M':
				multiplier = 1024 * 1024
				s = s[:len(s)-2]
			case 'G':
				multiplier = 1024 * 1024 * 1024
				s = s[:len(s)-2]
			default:
				s = s[:len(s)-1]
			}
		} else {
			s = s[:len(s)-1]
		}
	}

	var value int64
	fmt.Sscanf(s, "%d", &value)
	return value * multiplier
}

// parseQueryToSearchOptions converts a query string to SearchOptions
func parseQueryToSearchOptions(query string) imap.SearchOptions {
	from, subject, body := parseQueryString(query)
	return imap.SearchOptions{
		Query:   body,
		From:    from,
		Subject: subject,
	}
}

// Draft command handlers

func (c *DraftListCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
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

	ctx.Formatter.Verbosef("Fetching drafts...")

	drafts, err := client.ListDrafts(limit)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"mailbox":  "Drafts",
			"count":    len(drafts),
			"messages": drafts,
		})
	}

	if len(drafts) == 0 {
		fmt.Println("No drafts")
		return nil
	}

	tw := ctx.Formatter.NewTable("ID", "TO", "SUBJECT", "DATE")

	for _, d := range drafts {
		tw.AddRow(
			fmt.Sprintf("%d", d.SeqNum),
			d.From, // From field contains the draft's To header
			truncate(d.Subject, 40),
			d.Date,
		)
	}

	tw.Flush()
	return nil
}

func (c *DraftCreateCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	body := c.Body
	if body == "" {
		// Read from stdin if no body provided
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}
			body = string(data)
		}
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	draft := &imap.Draft{
		To:      c.To,
		CC:      c.CC,
		Subject: c.Subject,
		Body:    body,
	}

	uid, err := client.CreateDraft(draft)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"message": "Draft created",
			"uid":     uid,
		})
	}

	ctx.Formatter.PrintSuccess(fmt.Sprintf("Draft created (UID: %d)", uid))
	return nil
}

func (c *DraftEditCmd) Run(ctx *Context) error {
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

	// Get existing draft to merge with new values
	existing, err := client.GetDraft(c.ID)
	if err != nil {
		return fmt.Errorf("failed to get draft: %w", err)
	}

	// Use new values if provided, otherwise keep existing
	to := c.To
	if len(to) == 0 {
		to = existing.To
	}

	cc := c.CC
	if len(cc) == 0 {
		cc = existing.CC
	}

	subject := c.Subject
	if subject == "" {
		subject = existing.Subject
	}

	body := c.Body
	if body == "" {
		body = existing.TextBody
	}

	draft := &imap.Draft{
		To:      to,
		CC:      cc,
		Subject: subject,
		Body:    body,
	}

	uid, err := client.UpdateDraft(c.ID, draft)
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"message": "Draft updated",
			"uid":     uid,
		})
	}

	ctx.Formatter.PrintSuccess(fmt.Sprintf("Draft updated (UID: %d)", uid))
	return nil
}

func (c *DraftDeleteCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	if len(c.IDs) == 0 {
		return fmt.Errorf("no draft IDs specified")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	if err := client.DeleteDraft(c.IDs); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Deleted %d draft(s)", len(c.IDs)),
			"ids":     c.IDs,
		})
	}

	ctx.Formatter.PrintSuccess(fmt.Sprintf("Deleted %d draft(s)", len(c.IDs)))
	return nil
}

// truncate shortens a string to max length with ellipsis
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
