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
	UID     uint32 `json:"uid"`
	SeqNum  uint32 `json:"seq_num"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Seen    bool   `json:"seen"`
	Flagged bool   `json:"flagged"`
}

type Message struct {
	UID       uint32   `json:"uid"`
	SeqNum    uint32   `json:"seq_num"`
	MessageID string   `json:"message_id,omitempty"`
	From      string   `json:"from"`
	To        []string `json:"to"`
	CC        []string `json:"cc,omitempty"`
	Subject   string   `json:"subject"`
	Date      string   `json:"date"`
	Flags     []string `json:"flags"`
	TextBody  string   `json:"text_body,omitempty"`
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
