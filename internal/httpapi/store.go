package httpapi

import (
	"errors"
	"slices"
	"strings"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	accounts []Account
	messages []Message
}

func MockStore() *Store {
	mailboxes := defaultMailboxes()

	accounts := []Account{
		{ID: "personal", Email: "jooseho@gmail.com", Label: "개인 계정", Initials: "JS", Unread: 12, Storage: "6.2 / 15 GB", Mailboxes: mailboxes},
		{ID: "ops", Email: "ops@company.com", Label: "업무 계정", Initials: "OP", Unread: 7, Storage: "3.8 / 10 GB", Mailboxes: mailboxes},
	}

	messages := []Message{
		{
			MessageSummary: MessageSummary{
				ID: "m1", AccountID: "personal", MailboxID: "inbox", Sender: "이서연", SenderEmail: "seoyeon.lee@company.com",
				Initials: "이", Subject: "회의 자료 공유드립니다 (Q3 로드맵)", Snippet: "내일 회의 전에 검토 부탁드릴 자료입니다",
				Time: "오전 9:14", FullDate: "2026년 7월 2일 (목) 오전 9:14", Unread: true, Flagged: true, HasAttachment: true,
			},
			RemoteImagesBlocked: true,
			Headers: MessageHead{
				From: "이서연 <seoyeon.lee@company.com>", To: []string{"나 <jooseho@gmail.com>"},
				Date: "2026년 7월 2일 (목) 오전 9:14", Subject: "회의 자료 공유드립니다 (Q3 로드맵)",
			},
			TextBody: []string{
				"주호님, 안녕하세요.",
				"내일 오전 회의 전에 미리 검토해 주셨으면 하는 Q3 로드맵 초안을 공유드립니다.",
				"프론트엔드는 백엔드가 MIME을 파싱해 반환한 구조화된 본문만 렌더링합니다.",
			},
			Attachments: []Attachment{
				{Name: "Q3_Roadmap_v2.pdf", Size: "2.4 MB", Type: "pdf"},
				{Name: "timeline_chart.png", Size: "712 KB", Type: "image"},
			},
		},
		{
			MessageSummary: MessageSummary{
				ID: "m2", AccountID: "personal", MailboxID: "inbox", Sender: "GitHub", SenderEmail: "notifications@github.com",
				Initials: "GH", Subject: "[joomail/webmail] Fix MIME charset decoding (#142)", Snippet: "kimjh approved these changes · ready to merge",
				Time: "오전 8:52", FullDate: "2026년 7월 2일 (목) 오전 8:52", Unread: true,
			},
			Headers: MessageHead{
				From: "GitHub <notifications@github.com>", To: []string{"나 <jooseho@gmail.com>"},
				Date: "2026년 7월 2일 (목) 오전 8:52", Subject: "[joomail/webmail] Fix MIME charset decoding (#142)",
			},
			TextBody: []string{"Pull request #142 has been approved.", "Charset decoding fixtures cover EUC-KR and ISO-2022-JP samples."},
		},
		{
			MessageSummary: MessageSummary{
				ID: "m12", AccountID: "ops", MailboxID: "inbox", Sender: "Postfix Report", SenderEmail: "postmaster@company.com",
				Initials: "PR", Subject: "Daily mail delivery report", Snippet: "Received 1,204 · deferred 3 · bounced 0",
				Time: "오전 6:00", FullDate: "2026년 7월 2일 (목) 오전 6:00", Unread: true,
			},
			Headers: MessageHead{
				From: "Postfix Report <postmaster@company.com>", To: []string{"ops@company.com"},
				Date: "2026년 7월 2일 (목) 오전 6:00", Subject: "Daily mail delivery report",
			},
			TextBody: []string{"Daily delivery report is ready.", "Received 1,204 messages. Deferred 3. Bounced 0."},
		},
	}

	return &Store{accounts: accounts, messages: messages}
}

func defaultMailboxes() []Mailbox {
	return []Mailbox{
		{ID: "inbox", Label: "받은편지함", Kind: "inbox", Selectable: true, Unread: 12},
		{ID: "starred", Label: "중요 표시", Kind: "starred", Selectable: true},
		{ID: "sent", Label: "보낸편지함", Kind: "sent", Selectable: true},
		{ID: "drafts", Label: "임시보관함", Kind: "drafts", Selectable: true, Unread: 2},
		{ID: "archive", Label: "보관함", Kind: "archive", Selectable: true},
		{ID: "spam", Label: "스팸", Kind: "spam", Selectable: true, Unread: 3},
		{ID: "trash", Label: "휴지통", Kind: "trash", Selectable: true},
		{
			ID: "work", Label: "Work", Kind: "folder", Selectable: true,
			Children: []Mailbox{
				{ID: "clients", Label: "Clients", Kind: "folder", Selectable: true, Unread: 4},
				{ID: "internal", Label: "Internal", Kind: "folder", Selectable: true},
			},
		},
	}
}

func (s *Store) Accounts() []Account {
	return slices.Clone(s.accounts)
}

func (s *Store) AccountByEmail(email string) (Account, bool) {
	for _, account := range s.accounts {
		if strings.EqualFold(account.Email, email) {
			return account, true
		}
	}
	return Account{}, false
}

func (s *Store) MessageSummaries(accountID, mailboxID, query string) []MessageSummary {
	query = strings.ToLower(strings.TrimSpace(query))
	var summaries []MessageSummary
	for _, message := range s.messages {
		if message.AccountID != accountID || message.MailboxID != mailboxID {
			continue
		}
		if query != "" && !messageMatches(message, query) {
			continue
		}
		summaries = append(summaries, message.MessageSummary)
	}
	return summaries
}

func (s *Store) Message(id string) (Message, error) {
	for _, message := range s.messages {
		if message.ID == id {
			return message, nil
		}
	}
	return Message{}, ErrNotFound
}

func messageMatches(message Message, query string) bool {
	fields := []string{message.Sender, message.SenderEmail, message.Subject, message.Snippet}
	fields = append(fields, message.TextBody...)
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), query) {
			return true
		}
	}
	return false
}
