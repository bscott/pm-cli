package imap

import (
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bscott/pm-cli/internal/config"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type Client struct {
	client *imapclient.Client
	config *config.Config
}

func NewClient(cfg *config.Config) (*Client, error) {
	return &Client{
		config: cfg,
	}, nil
}

func (c *Client) Connect() error {
	password, err := c.config.GetPassword()
	if err != nil {
		return fmt.Errorf("failed to get password: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", c.config.Bridge.IMAPHost, c.config.Bridge.IMAPPort)

	// TLS config for STARTTLS - skip verification for Proton Bridge self-signed certs
	options := &imapclient.Options{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         c.config.Bridge.IMAPHost,
		},
	}

	// Connect with STARTTLS
	client, err := imapclient.DialStartTLS(addr, options)
	if err != nil {
		return fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	// Login
	if err := client.Login(c.config.Bridge.Email, password).Wait(); err != nil {
		client.Close()
		return fmt.Errorf("IMAP login failed: %w", err)
	}

	c.client = client
	return nil
}

func (c *Client) Close() error {
	if c.client != nil {
		if err := c.client.Logout().Wait(); err != nil {
			// Ignore logout errors, just close
		}
		return c.client.Close()
	}
	return nil
}

func (c *Client) ListMailboxes() ([]MailboxInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	listCmd := c.client.List("", "*", nil)
	mailboxes, err := listCmd.Collect()
	if err != nil {
		return nil, fmt.Errorf("failed to list mailboxes: %w", err)
	}

	var result []MailboxInfo
	for _, mb := range mailboxes {
		info := MailboxInfo{
			Name:       mb.Mailbox,
			Delimiter:  string(mb.Delim),
			Attributes: make([]string, 0, len(mb.Attrs)),
		}
		for _, attr := range mb.Attrs {
			info.Attributes = append(info.Attributes, string(attr))
		}
		result = append(result, info)
	}

	return result, nil
}

func (c *Client) SelectMailbox(name string) (*MailboxStatus, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	selected, err := c.client.Select(name, nil).Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to select mailbox %s: %w", name, err)
	}

	return &MailboxStatus{
		Name:     name,
		Messages: selected.NumMessages,
		Recent:   0, // Not available in v2 select response
		Unseen:   0, // Not available in v2 select response
	}, nil
}

func (c *Client) ListMessages(mailbox string, limit int, unreadOnly bool) ([]MessageSummary, error) {
	status, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, err
	}

	if status.Messages == 0 {
		return []MessageSummary{}, nil
	}

	// Calculate the range of messages to fetch (most recent first)
	start := uint32(1)
	if status.Messages > uint32(limit) {
		start = status.Messages - uint32(limit) + 1
	}

	// Build sequence set for range
	var seqSet imap.SeqSet
	seqSet.AddRange(start, status.Messages)

	// Fetch options
	fetchOptions := &imap.FetchOptions{
		UID:         true,
		Flags:       true,
		Envelope:    true,
		InternalDate: true,
	}

	fetchCmd := c.client.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	var messages []MessageSummary
	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		// Collect all data for this message
		var envelope *imap.Envelope
		var flags []imap.Flag
		var uid imap.UID
		var date string

		for {
			item := msg.Next()
			if item == nil {
				break
			}

			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				uid = data.UID
			case imapclient.FetchItemDataFlags:
				flags = data.Flags
			case imapclient.FetchItemDataEnvelope:
				envelope = data.Envelope
			case imapclient.FetchItemDataInternalDate:
				date = data.Time.Format("2006-01-02 15:04")
			}
		}

		if envelope == nil {
			continue
		}

		// Check if unread only
		seen := false
		flagged := false
		for _, f := range flags {
			if f == imap.FlagSeen {
				seen = true
			}
			if f == imap.FlagFlagged {
				flagged = true
			}
		}

		if unreadOnly && seen {
			continue
		}

		from := ""
		if len(envelope.From) > 0 {
			addr := envelope.From[0]
			if addr.Name != "" {
				from = addr.Name
			} else {
				from = addr.Addr()
			}
		}

		summary := MessageSummary{
			UID:     uint32(uid),
			SeqNum:  msg.SeqNum,
			From:    from,
			Subject: envelope.Subject,
			Date:    date,
			Seen:    seen,
			Flagged: flagged,
		}

		messages = append(messages, summary)
	}

	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	// Reverse to show newest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (c *Client) GetMessage(mailbox string, id string) (*Message, error) {
	status, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, err
	}

	if status.Messages == 0 {
		return nil, fmt.Errorf("mailbox is empty")
	}

	// Parse the ID as a sequence number
	var seqNum uint32
	if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
		return nil, fmt.Errorf("invalid message ID: %s", id)
	}

	seqSet := imap.SeqSetNum(seqNum)

	fetchOptions := &imap.FetchOptions{
		UID:          true,
		Flags:        true,
		Envelope:     true,
		InternalDate: true,
		BodySection:  []*imap.FetchItemBodySection{{}}, // Fetch full body
	}

	fetchCmd := c.client.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	msg := fetchCmd.Next()
	if msg == nil {
		return nil, fmt.Errorf("message not found: %s", id)
	}

	result := &Message{
		SeqNum: msg.SeqNum,
	}

	for {
		item := msg.Next()
		if item == nil {
			break
		}

		switch data := item.(type) {
		case imapclient.FetchItemDataUID:
			result.UID = uint32(data.UID)
		case imapclient.FetchItemDataFlags:
			result.Flags = make([]string, len(data.Flags))
			for i, f := range data.Flags {
				result.Flags[i] = string(f)
			}
		case imapclient.FetchItemDataEnvelope:
			result.Subject = data.Envelope.Subject
			result.Date = data.Envelope.Date.Format("2006-01-02 15:04:05")
			if len(data.Envelope.From) > 0 {
				addr := data.Envelope.From[0]
				result.From = formatAddress(addr)
			}
			for _, addr := range data.Envelope.To {
				result.To = append(result.To, formatAddress(addr))
			}
			for _, addr := range data.Envelope.Cc {
				result.CC = append(result.CC, formatAddress(addr))
			}
			result.MessageID = data.Envelope.MessageID
		case imapclient.FetchItemDataBodySection:
			body, err := readAll(data.Literal)
			if err == nil {
				result.RawBody = body
			}
		}
	}

	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	return result, nil
}

func (c *Client) DeleteMessages(mailbox string, ids []string, permanent bool) error {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return err
	}

	for _, id := range ids {
		var seqNum uint32
		if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
			return fmt.Errorf("invalid message ID: %s", id)
		}

		seqSet := imap.SeqSetNum(seqNum)

		// Add \Deleted flag
		storeCmd := c.client.Store(seqSet, &imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{imap.FlagDeleted},
		}, nil)
		if err := storeCmd.Close(); err != nil {
			return fmt.Errorf("failed to mark message %s for deletion: %w", id, err)
		}
	}

	// Expunge if permanent
	if permanent {
		if err := c.client.Expunge().Close(); err != nil {
			return fmt.Errorf("failed to expunge: %w", err)
		}
	}

	return nil
}

func (c *Client) MoveMessage(mailbox, id, destMailbox string) error {
	return c.MoveMessages(mailbox, []string{id}, destMailbox)
}

func (c *Client) MoveMessages(mailbox string, ids []string, destMailbox string) error {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return err
	}

	// Build sequence set from all IDs
	var seqSet imap.SeqSet
	for _, id := range ids {
		var seqNum uint32
		if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
			return fmt.Errorf("invalid message ID: %s", id)
		}
		seqSet.AddNum(seqNum)
	}

	// Copy to destination
	copyCmd := c.client.Copy(seqSet, destMailbox)
	if _, err := copyCmd.Wait(); err != nil {
		return fmt.Errorf("failed to copy messages to %s: %w", destMailbox, err)
	}

	// Delete from source
	storeCmd := c.client.Store(seqSet, &imap.StoreFlags{
		Op:    imap.StoreFlagsAdd,
		Flags: []imap.Flag{imap.FlagDeleted},
	}, nil)
	if err := storeCmd.Close(); err != nil {
		return fmt.Errorf("failed to delete from source: %w", err)
	}

	if err := c.client.Expunge().Close(); err != nil {
		return fmt.Errorf("failed to expunge: %w", err)
	}

	return nil
}

func (c *Client) SetFlags(mailbox, id string, read, unread, star, unstar bool) error {
	return c.SetFlagsMultiple(mailbox, []string{id}, read, unread, star, unstar)
}

func (c *Client) SetFlagsMultiple(mailbox string, ids []string, read, unread, star, unstar bool) error {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return err
	}

	// Build sequence set from all IDs
	var seqSet imap.SeqSet
	for _, id := range ids {
		var seqNum uint32
		if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
			return fmt.Errorf("invalid message ID: %s", id)
		}
		seqSet.AddNum(seqNum)
	}

	if read {
		storeCmd := c.client.Store(seqSet, &imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{imap.FlagSeen},
		}, nil)
		if err := storeCmd.Close(); err != nil {
			return fmt.Errorf("failed to mark as read: %w", err)
		}
	}

	if unread {
		storeCmd := c.client.Store(seqSet, &imap.StoreFlags{
			Op:    imap.StoreFlagsDel,
			Flags: []imap.Flag{imap.FlagSeen},
		}, nil)
		if err := storeCmd.Close(); err != nil {
			return fmt.Errorf("failed to mark as unread: %w", err)
		}
	}

	if star {
		storeCmd := c.client.Store(seqSet, &imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{imap.FlagFlagged},
		}, nil)
		if err := storeCmd.Close(); err != nil {
			return fmt.Errorf("failed to add star: %w", err)
		}
	}

	if unstar {
		storeCmd := c.client.Store(seqSet, &imap.StoreFlags{
			Op:    imap.StoreFlagsDel,
			Flags: []imap.Flag{imap.FlagFlagged},
		}, nil)
		if err := storeCmd.Close(); err != nil {
			return fmt.Errorf("failed to remove star: %w", err)
		}
	}

	return nil
}

func (c *Client) Search(mailbox string, opts SearchOptions) ([]MessageSummary, error) {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, err
	}

	// Build search criteria based on options
	criteria := c.buildSearchCriteria(opts)

	searchCmd := c.client.Search(criteria, nil)
	searchData, err := searchCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if len(searchData.AllSeqNums()) == 0 {
		return []MessageSummary{}, nil
	}

	// Fetch the matching messages
	seqSet := imap.SeqSetNum(searchData.AllSeqNums()...)

	fetchOptions := &imap.FetchOptions{
		UID:      true,
		Flags:    true,
		Envelope: true,
	}

	fetchCmd := c.client.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	var messages []MessageSummary
	for {
		msg := fetchCmd.Next()
		if msg == nil {
			break
		}

		var envelope *imap.Envelope
		var flags []imap.Flag
		var uid imap.UID

		for {
			item := msg.Next()
			if item == nil {
				break
			}

			switch data := item.(type) {
			case imapclient.FetchItemDataUID:
				uid = data.UID
			case imapclient.FetchItemDataFlags:
				flags = data.Flags
			case imapclient.FetchItemDataEnvelope:
				envelope = data.Envelope
			}
		}

		if envelope == nil {
			continue
		}

		seen := false
		flagged := false
		for _, f := range flags {
			if f == imap.FlagSeen {
				seen = true
			}
			if f == imap.FlagFlagged {
				flagged = true
			}
		}

		fromStr := ""
		if len(envelope.From) > 0 {
			addr := envelope.From[0]
			if addr.Name != "" {
				fromStr = addr.Name
			} else {
				fromStr = addr.Addr()
			}
		}

		summary := MessageSummary{
			UID:     uint32(uid),
			SeqNum:  msg.SeqNum,
			From:    fromStr,
			Subject: envelope.Subject,
			Date:    envelope.Date.Format("2006-01-02 15:04"),
			Seen:    seen,
			Flagged: flagged,
		}

		messages = append(messages, summary)
	}

	return messages, fetchCmd.Close()
}

// SearchIDs returns sequence numbers of messages matching the search criteria.
// This is used for batch operations based on queries.
func (c *Client) SearchIDs(mailbox string, opts SearchOptions) ([]string, error) {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, err
	}

	// Build search criteria
	criteria := c.buildSearchCriteria(opts)

	searchCmd := c.client.Search(criteria, nil)
	searchData, err := searchCmd.Wait()
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	seqNums := searchData.AllSeqNums()
	ids := make([]string, len(seqNums))
	for i, num := range seqNums {
		ids[i] = fmt.Sprintf("%d", num)
	}

	return ids, nil
}

// buildSearchCriteria constructs IMAP search criteria from SearchOptions
func (c *Client) buildSearchCriteria(opts SearchOptions) *imap.SearchCriteria {
	// For OR logic, we need to build individual criteria and combine them
	if opts.UseOr {
		return c.buildOrSearchCriteria(opts)
	}

	// Default AND logic: all criteria go into a single SearchCriteria
	criteria := &imap.SearchCriteria{}

	// Body text search (general query or explicit body search)
	if opts.Query != "" {
		criteria.Body = append(criteria.Body, opts.Query)
	}
	if opts.Body != "" {
		criteria.Body = append(criteria.Body, opts.Body)
	}

	// Header filters
	if opts.From != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "From",
			Value: opts.From,
		})
	}

	if opts.To != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "To",
			Value: opts.To,
		})
	}

	if opts.Subject != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "Subject",
			Value: opts.Subject,
		})
	}

	// Date filters
	if opts.Since != "" {
		if t, err := parseDate(opts.Since); err == nil {
			criteria.Since = t
		}
	}

	if opts.Before != "" {
		if t, err := parseDate(opts.Before); err == nil {
			criteria.Before = t
		}
	}

	// Size filters
	if opts.LargerThan > 0 {
		criteria.Larger = opts.LargerThan
	}

	if opts.SmallerThan > 0 {
		criteria.Smaller = opts.SmallerThan
	}

	// Attachment filter: IMAP doesn't have a direct "has attachment" search key,
	// but multipart/mixed Content-Type typically indicates attachments
	if opts.HasAttachments {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "Content-Type",
			Value: "multipart/mixed",
		})
	}

	// Negation: wrap the entire criteria in a NOT
	if opts.Negate {
		return &imap.SearchCriteria{
			Not: []imap.SearchCriteria{*criteria},
		}
	}

	return criteria
}

// buildOrSearchCriteria constructs criteria with OR logic between filters
func (c *Client) buildOrSearchCriteria(opts SearchOptions) *imap.SearchCriteria {
	var orCriteria []imap.SearchCriteria

	// Build individual criteria for each filter
	if opts.Query != "" {
		orCriteria = append(orCriteria, imap.SearchCriteria{
			Body: []string{opts.Query},
		})
	}

	if opts.Body != "" {
		orCriteria = append(orCriteria, imap.SearchCriteria{
			Body: []string{opts.Body},
		})
	}

	if opts.From != "" {
		orCriteria = append(orCriteria, imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "From", Value: opts.From}},
		})
	}

	if opts.To != "" {
		orCriteria = append(orCriteria, imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "To", Value: opts.To}},
		})
	}

	if opts.Subject != "" {
		orCriteria = append(orCriteria, imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "Subject", Value: opts.Subject}},
		})
	}

	if opts.Since != "" {
		if t, err := parseDate(opts.Since); err == nil {
			orCriteria = append(orCriteria, imap.SearchCriteria{Since: t})
		}
	}

	if opts.Before != "" {
		if t, err := parseDate(opts.Before); err == nil {
			orCriteria = append(orCriteria, imap.SearchCriteria{Before: t})
		}
	}

	if opts.LargerThan > 0 {
		orCriteria = append(orCriteria, imap.SearchCriteria{Larger: opts.LargerThan})
	}

	if opts.SmallerThan > 0 {
		orCriteria = append(orCriteria, imap.SearchCriteria{Smaller: opts.SmallerThan})
	}

	if opts.HasAttachments {
		orCriteria = append(orCriteria, imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "Content-Type", Value: "multipart/mixed"}},
		})
	}

	// If no criteria, return empty criteria (matches all)
	if len(orCriteria) == 0 {
		return &imap.SearchCriteria{}
	}

	// If only one criterion, return it directly
	if len(orCriteria) == 1 {
		if opts.Negate {
			return &imap.SearchCriteria{Not: orCriteria}
		}
		return &orCriteria[0]
	}

	// Combine with OR logic: IMAP OR takes two operands, so we chain them
	// OR(a, OR(b, OR(c, d))) for multiple criteria
	result := orCriteria[len(orCriteria)-1]
	for i := len(orCriteria) - 2; i >= 0; i-- {
		result = imap.SearchCriteria{
			Or: [][2]imap.SearchCriteria{{orCriteria[i], result}},
		}
	}

	if opts.Negate {
		return &imap.SearchCriteria{Not: []imap.SearchCriteria{result}}
	}

	return &result
}

// parseDate parses a date string in YYYY-MM-DD format
func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

func (c *Client) CreateMailbox(name string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	if err := c.client.Create(name, nil).Wait(); err != nil {
		return fmt.Errorf("failed to create mailbox %s: %w", name, err)
	}

	return nil
}

func (c *Client) DeleteMailbox(name string) error {
	if c.client == nil {
		return fmt.Errorf("not connected")
	}

	if err := c.client.Delete(name).Wait(); err != nil {
		return fmt.Errorf("failed to delete mailbox %s: %w", name, err)
	}

	return nil
}

func formatAddress(addr imap.Address) string {
	if addr.Name != "" {
		return fmt.Sprintf("%s <%s>", addr.Name, addr.Addr())
	}
	return addr.Addr()
}

func readAll(r imap.LiteralReader) ([]byte, error) {
	return io.ReadAll(r)
}

// GetAttachments returns a list of attachments for a message without downloading the data
func (c *Client) GetAttachments(mailbox, id string) ([]Attachment, error) {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, err
	}

	var seqNum uint32
	if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
		return nil, fmt.Errorf("invalid message ID: %s", id)
	}

	seqSet := imap.SeqSetNum(seqNum)

	// Fetch BODYSTRUCTURE to get attachment info
	fetchOptions := &imap.FetchOptions{
		BodyStructure: &imap.FetchItemBodyStructure{},
	}

	fetchCmd := c.client.Fetch(seqSet, fetchOptions)
	defer fetchCmd.Close()

	msg := fetchCmd.Next()
	if msg == nil {
		return nil, fmt.Errorf("message not found: %s", id)
	}

	var attachments []Attachment
	index := 0

	for {
		item := msg.Next()
		if item == nil {
			break
		}

		if data, ok := item.(imapclient.FetchItemDataBodyStructure); ok {
			attachments = extractAttachments(data.BodyStructure, &index, "")
		}
	}

	if err := fetchCmd.Close(); err != nil {
		return nil, fmt.Errorf("fetch failed: %w", err)
	}

	return attachments, nil
}

// extractAttachments recursively extracts attachment info from body structure
func extractAttachments(bs imap.BodyStructure, index *int, partNum string) []Attachment {
	var attachments []Attachment

	switch s := bs.(type) {
	case *imap.BodyStructureSinglePart:
		// Check if this is an attachment (has filename or is not text/html)
		filename := ""
		disp := s.Disposition()
		if disp != nil {
			if name, ok := disp.Params["filename"]; ok {
				filename = name
			}
		}
		// Check Content-Type params for name
		if filename == "" {
			if name, ok := s.Params["name"]; ok {
				filename = name
			}
		}

		// Include as attachment if it has a filename or disposition is attachment
		isAttachment := filename != "" ||
			(disp != nil && disp.Value == "attachment")

		// Also include non-text parts that aren't inline
		if !isAttachment && s.Type != "text" {
			isAttachment = disp == nil || disp.Value != "inline"
		}

		if isAttachment {
			att := Attachment{
				Index:       *index,
				Filename:    filename,
				ContentType: fmt.Sprintf("%s/%s", s.Type, s.Subtype),
				Size:        int64(s.Size),
			}
			if att.Filename == "" {
				att.Filename = fmt.Sprintf("attachment_%d", *index)
			}
			attachments = append(attachments, att)
			*index++
		}

	case *imap.BodyStructureMultiPart:
		for i, child := range s.Children {
			childPart := fmt.Sprintf("%d", i+1)
			if partNum != "" {
				childPart = fmt.Sprintf("%s.%d", partNum, i+1)
			}
			attachments = append(attachments, extractAttachments(child, index, childPart)...)
		}
	}

	return attachments
}

// DownloadAttachment downloads a specific attachment by index
func (c *Client) DownloadAttachment(mailbox, id string, index int) ([]byte, string, error) {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, "", err
	}

	var seqNum uint32
	if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
		return nil, "", fmt.Errorf("invalid message ID: %s", id)
	}

	seqSet := imap.SeqSetNum(seqNum)

	// First get the body structure to find the attachment part
	fetchOptions := &imap.FetchOptions{
		BodyStructure: &imap.FetchItemBodyStructure{},
	}

	fetchCmd := c.client.Fetch(seqSet, fetchOptions)

	msg := fetchCmd.Next()
	if msg == nil {
		fetchCmd.Close()
		return nil, "", fmt.Errorf("message not found: %s", id)
	}

	var bodyStruct imap.BodyStructure
	for {
		item := msg.Next()
		if item == nil {
			break
		}
		if data, ok := item.(imapclient.FetchItemDataBodyStructure); ok {
			bodyStruct = data.BodyStructure
		}
	}
	fetchCmd.Close()

	if bodyStruct == nil {
		return nil, "", fmt.Errorf("could not get message structure")
	}

	// Find the attachment part number
	partInfo := findAttachmentPart(bodyStruct, index, "", 0)
	if partInfo == nil {
		return nil, "", fmt.Errorf("attachment index %d not found", index)
	}

	// Fetch the specific part
	section := &imap.FetchItemBodySection{
		Part: partInfo.partNums,
	}

	fetchOptions2 := &imap.FetchOptions{
		BodySection: []*imap.FetchItemBodySection{section},
	}

	fetchCmd2 := c.client.Fetch(seqSet, fetchOptions2)
	defer fetchCmd2.Close()

	msg2 := fetchCmd2.Next()
	if msg2 == nil {
		return nil, "", fmt.Errorf("message not found: %s", id)
	}

	var data []byte
	for {
		item := msg2.Next()
		if item == nil {
			break
		}
		if bodyData, ok := item.(imapclient.FetchItemDataBodySection); ok {
			data, _ = io.ReadAll(bodyData.Literal)
		}
	}

	if err := fetchCmd2.Close(); err != nil {
		return nil, "", fmt.Errorf("fetch failed: %w", err)
	}

	return data, partInfo.filename, nil
}

type attachmentPartInfo struct {
	partNums []int
	filename string
}

// findAttachmentPart finds the MIME part numbers for the attachment at the given index
func findAttachmentPart(bs imap.BodyStructure, targetIndex int, partNum string, currentIndex int) *attachmentPartInfo {
	switch s := bs.(type) {
	case *imap.BodyStructureSinglePart:
		filename := ""
		disp := s.Disposition()
		if disp != nil {
			if name, ok := disp.Params["filename"]; ok {
				filename = name
			}
		}
		if filename == "" {
			if name, ok := s.Params["name"]; ok {
				filename = name
			}
		}

		isAttachment := filename != "" ||
			(disp != nil && disp.Value == "attachment")

		if !isAttachment && s.Type != "text" {
			isAttachment = disp == nil || disp.Value != "inline"
		}

		if isAttachment {
			if currentIndex == targetIndex {
				if filename == "" {
					filename = fmt.Sprintf("attachment_%d", targetIndex)
				}
				// Parse part numbers from partNum string
				var partNums []int
				if partNum != "" {
					parts := splitPartNum(partNum)
					partNums = parts
				}
				return &attachmentPartInfo{
					partNums: partNums,
					filename: filename,
				}
			}
		}

	case *imap.BodyStructureMultiPart:
		idx := currentIndex
		for i, child := range s.Children {
			childPart := fmt.Sprintf("%d", i+1)
			if partNum != "" {
				childPart = fmt.Sprintf("%s.%d", partNum, i+1)
			}

			// Count attachments in this child
			childCount := countAttachments(child)

			result := findAttachmentPart(child, targetIndex, childPart, idx)
			if result != nil {
				return result
			}
			idx += childCount
		}
	}

	return nil
}

// countAttachments counts the number of attachments in a body structure
func countAttachments(bs imap.BodyStructure) int {
	count := 0
	switch s := bs.(type) {
	case *imap.BodyStructureSinglePart:
		filename := ""
		disp := s.Disposition()
		if disp != nil {
			if name, ok := disp.Params["filename"]; ok {
				filename = name
			}
		}
		if filename == "" {
			if name, ok := s.Params["name"]; ok {
				filename = name
			}
		}

		isAttachment := filename != "" ||
			(disp != nil && disp.Value == "attachment")

		if !isAttachment && s.Type != "text" {
			isAttachment = disp == nil || disp.Value != "inline"
		}

		if isAttachment {
			count++
		}

	case *imap.BodyStructureMultiPart:
		for _, child := range s.Children {
			count += countAttachments(child)
		}
	}
	return count
}

// splitPartNum converts "1.2.3" to []int{1, 2, 3}
func splitPartNum(s string) []int {
	if s == "" {
		return nil
	}
	var parts []int
	for _, p := range strings.Split(s, ".") {
		var n int
		fmt.Sscanf(p, "%d", &n)
		parts = append(parts, n)
	}
	return parts
}
