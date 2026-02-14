package cli

import (
	"fmt"

	"github.com/bscott/pm-cli/internal/imap"
)

func (c *MailboxListCmd) Run(ctx *Context) error {
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

	ctx.Formatter.Verbosef("Listing mailboxes...")

	mailboxes, err := client.ListMailboxes()
	if err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"count":     len(mailboxes),
			"mailboxes": mailboxes,
		})
	}

	if len(mailboxes) == 0 {
		fmt.Println("No mailboxes found.")
		return nil
	}

	fmt.Printf("Mailboxes (%d):\n\n", len(mailboxes))

	for _, mb := range mailboxes {
		attrs := ""
		if len(mb.Attributes) > 0 {
			attrs = fmt.Sprintf(" [%s]", formatAttributes(mb.Attributes))
		}
		fmt.Printf("  %s%s\n", mb.Name, attrs)
	}

	return nil
}

func (c *MailboxCreateCmd) Run(ctx *Context) error {
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

	if err := client.CreateMailbox(c.Name); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"mailbox": c.Name,
			"message": "Mailbox created",
		})
	}

	fmt.Printf("Mailbox '%s' created.\n", c.Name)
	return nil
}

func (c *MailboxDeleteCmd) Run(ctx *Context) error {
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

	if err := client.DeleteMailbox(c.Name); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"mailbox": c.Name,
			"message": "Mailbox deleted",
		})
	}

	fmt.Printf("Mailbox '%s' deleted.\n", c.Name)
	return nil
}

func formatAttributes(attrs []string) string {
	if len(attrs) == 0 {
		return ""
	}

	// Clean up attribute names (remove backslashes)
	cleaned := make([]string, len(attrs))
	for i, attr := range attrs {
		if len(attr) > 0 && attr[0] == '\\' {
			cleaned[i] = attr[1:]
		} else {
			cleaned[i] = attr
		}
	}

	result := ""
	for i, attr := range cleaned {
		if i > 0 {
			result += ", "
		}
		result += attr
	}
	return result
}
