package imap

type MailboxInfo struct {
	Name       string   `json:"name"`
	Delimiter  string   `json:"delimiter"`
	Attributes []string `json:"attributes"`
}

type MailboxStatus struct {
	Name     string `json:"name"`
	Messages uint32 `json:"messages"`
	Recent   uint32 `json:"recent"`
	Unseen   uint32 `json:"unseen"`
}

type MessageSummary struct {
	UID uint32 `json:"uid"`
	// SeqNum is the message sequence number (the "ID" shown in text output).
	SeqNum uint32 `json:"seq_num"`
	// From is the sender display name when present, otherwise the address. It
	// is kept for backward compatibility; FromAddress always carries the bare
	// email address.
	From string `json:"from"`
	// FromAddress is the sender's email address (foo@example.org), useful for
	// matching on sender/domain without a per-message `mail read`.
	FromAddress string `json:"from_address,omitempty"`
	// To lists the recipient addresses from the envelope.
	To      []string `json:"to,omitempty"`
	Subject string   `json:"subject"`
	// MessageID is the RFC 5322 Message-ID header.
	MessageID string `json:"message_id,omitempty"`
	// InReplyTo is the RFC 5322 In-Reply-To header, useful for reconstructing
	// reply chains directly from list output.
	InReplyTo string `json:"in_reply_to,omitempty"`
	Date      string `json:"date"`
	DateISO   string `json:"date_iso,omitempty"`
	Seen      bool   `json:"seen"`
	Flagged   bool   `json:"flagged"`
}

// ListOptions controls how ListMessages selects and paginates messages.
// UnreadOnly and FlaggedOnly are applied server-side via IMAP SEARCH so the
// returned count respects Limit (client-side filtering could return fewer than
// Limit results). When neither filter is set, the most recent Limit messages
// are returned (honoring Offset).
type ListOptions struct {
	Mailbox     string
	Limit       int
	Offset      int
	UnreadOnly  bool // SEARCH UNSEEN
	FlaggedOnly bool // SEARCH FLAGGED
}

type Message struct {
	UID         uint32       `json:"uid"`
	SeqNum      uint32       `json:"seq_num"`
	MessageID   string       `json:"message_id,omitempty"`
	InReplyTo   string       `json:"in_reply_to,omitempty"`
	References  []string     `json:"references,omitempty"`
	From        string       `json:"from"`
	To          []string     `json:"to"`
	CC          []string     `json:"cc,omitempty"`
	Subject     string       `json:"subject"`
	Date        string       `json:"date"`
	DateISO     string       `json:"date_iso,omitempty"`
	Flags       []string     `json:"flags"`
	Labels      []string     `json:"labels,omitempty"`
	TextBody    string       `json:"text_body,omitempty"`
	HTMLBody    string       `json:"html_body,omitempty"`
	RawBody     []byte       `json:"-"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Data        []byte `json:"-"`
}

// SearchOptions contains all search criteria for the Search method
type SearchOptions struct {
	Query          string // General query (searches body text)
	From           string // Filter by sender
	To             string // Filter by recipient
	Subject        string // Filter by subject
	Body           string // Search in message body
	Since          string // Messages since date (YYYY-MM-DD)
	Before         string // Messages before date (YYYY-MM-DD)
	HasAttachments bool   // Only messages with attachments
	LargerThan     int64  // Messages larger than this size in bytes
	SmallerThan    int64  // Messages smaller than this size in bytes
	UseOr          bool   // Combine filters with OR instead of AND
	Negate         bool   // Negate the entire search
}
