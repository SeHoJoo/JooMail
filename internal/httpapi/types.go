package httpapi

type Mailbox struct {
	ID         string    `json:"id"`
	Label      string    `json:"label"`
	Kind       string    `json:"kind"`
	Selectable bool      `json:"selectable"`
	Unread     int       `json:"unread,omitempty"`
	Children   []Mailbox `json:"children,omitempty"`
}

type Account struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Label     string    `json:"label"`
	Initials  string    `json:"initials"`
	Unread    int       `json:"unread"`
	Storage   string    `json:"storage"`
	Mailboxes []Mailbox `json:"mailboxes"`
}

type Attachment struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Size string `json:"size"`
	Type string `json:"type"`
}

type MessageSummary struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	MailboxID     string `json:"mailboxId"`
	ThreadID      string `json:"threadId,omitempty"`
	Sender        string `json:"sender"`
	SenderEmail   string `json:"senderEmail"`
	Initials      string `json:"initials"`
	Subject       string `json:"subject"`
	Snippet       string `json:"snippet"`
	Time          string `json:"time"`
	FullDate      string `json:"fullDate"`
	Unread        bool   `json:"unread"`
	Flagged       bool   `json:"flagged,omitempty"`
	HasAttachment bool   `json:"hasAttachment,omitempty"`
}

type Message struct {
	MessageSummary
	RemoteImagesBlocked bool         `json:"remoteImagesBlocked,omitempty"`
	Headers             MessageHead  `json:"headers"`
	TextBody            []string     `json:"textBody"`
	HTMLBody            string       `json:"htmlBody,omitempty"`
	Attachments         []Attachment `json:"attachments,omitempty"`
}

type MessageHead struct {
	From       string   `json:"from"`
	To         []string `json:"to"`
	Cc         []string `json:"cc,omitempty"`
	Date       string   `json:"date"`
	Subject    string   `json:"subject"`
	MessageID  string   `json:"messageId,omitempty"`
	InReplyTo  string   `json:"inReplyTo,omitempty"`
	References []string `json:"references,omitempty"`
}

type MailRule struct {
	Name      string        `json:"name,omitempty"`
	Condition RuleCondition `json:"condition"`
	Action    RuleAction    `json:"action"`
}

type RuleCondition struct {
	Field string `json:"field"`
	Match string `json:"match"`
	Value string `json:"value"`
}

type RuleAction struct {
	Type      string `json:"type"`
	MailboxID string `json:"mailboxId,omitempty"`
}
