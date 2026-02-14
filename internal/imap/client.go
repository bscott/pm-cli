package imap

import (
	"crypto/tls"
	"fmt"

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
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return err
	}

	var seqNum uint32
	if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
		return fmt.Errorf("invalid message ID: %s", id)
	}

	seqSet := imap.SeqSetNum(seqNum)

	// Copy to destination
	copyCmd := c.client.Copy(seqSet, destMailbox)
	if _, err := copyCmd.Wait(); err != nil {
		return fmt.Errorf("failed to copy message to %s: %w", destMailbox, err)
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
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return err
	}

	var seqNum uint32
	if _, err := fmt.Sscanf(id, "%d", &seqNum); err != nil {
		return fmt.Errorf("invalid message ID: %s", id)
	}

	seqSet := imap.SeqSetNum(seqNum)

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

func (c *Client) Search(mailbox, query, from, subject, since, before string) ([]MessageSummary, error) {
	_, err := c.SelectMailbox(mailbox)
	if err != nil {
		return nil, err
	}

	// Build search criteria
	criteria := &imap.SearchCriteria{}

	if query != "" {
		criteria.Body = []string{query}
	}

	if from != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "From",
			Value: from,
		})
	}

	if subject != "" {
		criteria.Header = append(criteria.Header, imap.SearchCriteriaHeaderField{
			Key:   "Subject",
			Value: subject,
		})
	}

	// Note: Date filtering would require parsing the since/before strings
	// and setting criteria.Since / criteria.Before

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
	data := make([]byte, r.Size())
	_, err := r.Read(data)
	return data, err
}
