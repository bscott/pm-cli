package cli

import (
	"fmt"
	"strings"

	"github.com/bscott/pm-cli/internal/imap"
)

const labelPrefix = "Labels/"

// Run lists all available labels. In Proton Bridge, labels are exposed as
// folders under the "Labels/" parent folder.
func (c *LabelListCmd) Run(ctx *Context) error {
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

	ctx.Formatter.Verbosef("Listing labels...")

	mailboxes, err := client.ListMailboxes()
	if err != nil {
		return err
	}

	// Filter to only show labels (folders under Labels/)
	var labels []LabelInfo
	for _, mb := range mailboxes {
		if strings.HasPrefix(mb.Name, labelPrefix) {
			labelName := strings.TrimPrefix(mb.Name, labelPrefix)
			if labelName != "" {
				labels = append(labels, LabelInfo{
					Name:     labelName,
					FullPath: mb.Name,
				})
			}
		}
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"count":  len(labels),
			"labels": labels,
		})
	}

	if len(labels) == 0 {
		fmt.Println("No labels found.")
		fmt.Println("\nNote: Proton Mail labels appear as folders under 'Labels/'.")
		fmt.Println("Create labels in the Proton Mail web interface or mobile app.")
		return nil
	}

	fmt.Printf("Labels (%d):\n\n", len(labels))
	for _, label := range labels {
		fmt.Printf("  %s\n", label.Name)
	}

	return nil
}

// LabelInfo represents a Proton Mail label.
type LabelInfo struct {
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
}

// Run adds a label to message(s) by copying them to the label folder.
// In Proton Bridge, adding a label is done by copying the message to the
// corresponding Labels/LabelName folder.
func (c *LabelAddCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	if len(c.IDs) == 0 {
		return fmt.Errorf("no message IDs specified")
	}

	if c.Label == "" {
		return fmt.Errorf("no label specified - use --label or -l")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	// Build full label path
	labelPath := labelPrefix + c.Label

	// Verify the label exists
	mailboxes, err := client.ListMailboxes()
	if err != nil {
		return err
	}

	labelExists := false
	for _, mb := range mailboxes {
		if mb.Name == labelPath {
			labelExists = true
			break
		}
	}

	if !labelExists {
		return fmt.Errorf("label '%s' does not exist. Use 'pm-cli mail label list' to see available labels", c.Label)
	}

	ctx.Formatter.Verbosef("Adding label '%s' to %d message(s)...", c.Label, len(c.IDs))

	// Copy messages to the label folder (this adds the label without removing from source)
	if err := client.CopyMessages(c.Mailbox, c.IDs, labelPath); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"label":   c.Label,
			"ids":     c.IDs,
			"count":   len(c.IDs),
			"message": fmt.Sprintf("Label '%s' added to %d message(s)", c.Label, len(c.IDs)),
		})
	}

	fmt.Printf("Label '%s' added to %d message(s).\n", c.Label, len(c.IDs))
	return nil
}

// Run removes a label from message(s) by deleting them from the label folder.
// The message must be accessed from within the label folder to remove it.
func (c *LabelRemoveCmd) Run(ctx *Context) error {
	if ctx.Config.Bridge.Email == "" {
		return fmt.Errorf("not configured - run 'pm-cli config init' first")
	}

	if len(c.IDs) == 0 {
		return fmt.Errorf("no message IDs specified")
	}

	if c.Label == "" {
		return fmt.Errorf("no label specified - use --label or -l")
	}

	client, err := imap.NewClient(ctx.Config)
	if err != nil {
		return err
	}

	if err := client.Connect(); err != nil {
		return err
	}
	defer client.Close()

	// Build full label path
	labelPath := labelPrefix + c.Label

	ctx.Formatter.Verbosef("Removing label '%s' from %d message(s)...", c.Label, len(c.IDs))

	// Delete messages from the label folder. This removes the label but keeps the
	// message in its primary folder (INBOX, Archive, etc.)
	if err := client.DeleteMessages(labelPath, c.IDs, true); err != nil {
		return fmt.Errorf("failed to remove label: %w", err)
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"label":   c.Label,
			"ids":     c.IDs,
			"count":   len(c.IDs),
			"message": fmt.Sprintf("Label '%s' removed from %d message(s)", c.Label, len(c.IDs)),
		})
	}

	fmt.Printf("Label '%s' removed from %d message(s).\n", c.Label, len(c.IDs))
	return nil
}
