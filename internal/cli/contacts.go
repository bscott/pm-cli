package cli

import (
	"fmt"

	"github.com/bscott/pm-cli/internal/contacts"
)

func (c *ContactsListCmd) Run(ctx *Context) error {
	store, err := contacts.Load()
	if err != nil {
		return err
	}

	contactList := store.List()

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"contacts": contactList,
			"count":    len(contactList),
		})
	}

	if len(contactList) == 0 {
		fmt.Println("No contacts in address book.")
		fmt.Println("Use 'pm-cli contacts add <email>' to add contacts.")
		return nil
	}

	fmt.Printf("Contacts (%d):\n\n", len(contactList))

	table := ctx.Formatter.NewTable("EMAIL", "NAME")
	for _, contact := range contactList {
		table.AddRow(contact.Email, contact.Name)
	}
	table.Flush()

	return nil
}

func (c *ContactsSearchCmd) Run(ctx *Context) error {
	store, err := contacts.Load()
	if err != nil {
		return err
	}

	results := store.Search(c.Query)

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"query":    c.Query,
			"contacts": results,
			"count":    len(results),
		})
	}

	if len(results) == 0 {
		fmt.Printf("No contacts matching '%s'.\n", c.Query)
		return nil
	}

	fmt.Printf("Contacts matching '%s' (%d):\n\n", c.Query, len(results))

	table := ctx.Formatter.NewTable("EMAIL", "NAME")
	for _, contact := range results {
		table.AddRow(contact.Email, contact.Name)
	}
	table.Flush()

	return nil
}

func (c *ContactsAddCmd) Run(ctx *Context) error {
	store, err := contacts.Load()
	if err != nil {
		return err
	}

	if err := store.Add(c.Email, c.Name); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"message": "Contact added",
			"email":   c.Email,
			"name":    c.Name,
		})
	}

	if c.Name != "" {
		fmt.Printf("Added contact: %s <%s>\n", c.Name, c.Email)
	} else {
		fmt.Printf("Added contact: %s\n", c.Email)
	}

	return nil
}

func (c *ContactsRemoveCmd) Run(ctx *Context) error {
	store, err := contacts.Load()
	if err != nil {
		return err
	}

	// Get contact info before removal for display
	contact := store.Get(c.Email)
	if contact == nil {
		return fmt.Errorf("contact with email %s not found", c.Email)
	}

	if err := store.Remove(c.Email); err != nil {
		return err
	}

	if ctx.Formatter.JSON {
		return ctx.Formatter.PrintJSON(map[string]interface{}{
			"success": true,
			"message": "Contact removed",
			"email":   c.Email,
		})
	}

	if contact.Name != "" {
		fmt.Printf("Removed contact: %s <%s>\n", contact.Name, contact.Email)
	} else {
		fmt.Printf("Removed contact: %s\n", contact.Email)
	}

	return nil
}
