package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
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
		textBody, _ := parseMessageBody(msg.RawBody)
		if textBody != "" {
			fmt.Println(textBody)
		} else {
			fmt.Println("[No text body available]")
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

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		contentType := part.Header.Get("Content-Type")

		switch {
		case strings.HasPrefix(contentType, "text/plain"):
			body, err := io.ReadAll(part.Body)
			if err == nil {
				textBody = string(body)
			}
		case strings.HasPrefix(contentType, "text/html"):
			body, err := io.ReadAll(part.Body)
			if err == nil {
				htmlBody = string(body)
			}
		}
	}

	return textBody, htmlBody
}
