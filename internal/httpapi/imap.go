package httpapi

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"
)

type imapClient struct {
	conn   io.ReadWriteCloser
	reader *bufio.Reader
	next   int
}

type imapResponse struct {
	line    string
	literal []byte
}

func openIMAPSession(config Config, credential storedCredential) (*imapClient, error) {
	conn, err := dialIMAP(config)
	if err != nil {
		return nil, err
	}
	client := &imapClient{conn: conn, reader: bufio.NewReader(conn)}
	if _, err := client.reader.ReadString('\n'); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := client.login(credential.IMAPUsername, credential.Password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

func (c *imapClient) Close() error {
	_, _ = c.command("LOGOUT")
	return c.conn.Close()
}

func (c *imapClient) login(username string, password string) error {
	responses, err := c.command("LOGIN %s %s", quoteIMAPString(username), quoteIMAPString(password))
	if err != nil {
		return err
	}
	status := taggedStatus(responses)
	if status != "OK" {
		return errInvalidCredentials
	}
	return nil
}

func (c *imapClient) listMailboxes() ([]Mailbox, error) {
	responses, err := c.command(`LIST "" "*"`)
	if err != nil {
		return nil, err
	}
	var mailboxes []Mailbox
	for _, response := range responses {
		if !strings.HasPrefix(response.line, "* LIST ") {
			continue
		}
		name := parseLastQuoted(response.line)
		if name == "" {
			continue
		}
		mailboxes = append(mailboxes, mailboxFromIMAPName(name))
	}
	if len(mailboxes) == 0 {
		mailboxes = append(mailboxes, mailboxFromIMAPName("INBOX"))
	}
	return mailboxes, nil
}

func (c *imapClient) messageSummaries(accountID string, mailboxID string, query string) ([]MessageSummary, error) {
	mailboxName, err := decodeMailboxID(mailboxID)
	if err != nil {
		return nil, err
	}
	uids, err := c.searchMailbox(mailboxName)
	if err != nil {
		return nil, err
	}
	if len(uids) > 50 {
		uids = uids[:50]
	}
	messages, err := c.fetchMessages(accountID, mailboxID, mailboxName, uids)
	if err != nil {
		return nil, err
	}
	query = strings.ToLower(strings.TrimSpace(query))
	summaries := make([]MessageSummary, 0, len(messages))
	for _, message := range messages {
		if query != "" && !messageMatches(message, query) {
			continue
		}
		summaries = append(summaries, message.MessageSummary)
	}
	return summaries, nil
}

func (c *imapClient) message(accountID string, messageID string) (Message, error) {
	mailboxID, uid, err := decodeMessageID(messageID)
	if err != nil {
		return Message{}, ErrNotFound
	}
	mailboxName, err := decodeMailboxID(mailboxID)
	if err != nil {
		return Message{}, ErrNotFound
	}
	messages, err := c.fetchMessages(accountID, mailboxID, mailboxName, []string{uid})
	if err != nil {
		return Message{}, err
	}
	if len(messages) == 0 {
		return Message{}, ErrNotFound
	}
	return messages[0], nil
}

func (c *imapClient) searchMailbox(mailboxName string) ([]string, error) {
	if responses, err := c.command("SELECT %s", quoteIMAPString(mailboxName)); err != nil {
		return nil, err
	} else if taggedStatus(responses) != "OK" {
		return nil, errors.New("select failed")
	}
	responses, err := c.command("UID SEARCH ALL")
	if err != nil {
		return nil, err
	}
	var uids []int
	for _, response := range responses {
		if !strings.HasPrefix(response.line, "* SEARCH") {
			continue
		}
		for _, field := range strings.Fields(strings.TrimPrefix(response.line, "* SEARCH")) {
			uid, err := strconv.Atoi(field)
			if err == nil {
				uids = append(uids, uid)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(uids)))
	out := make([]string, len(uids))
	for i, uid := range uids {
		out[i] = strconv.Itoa(uid)
	}
	return out, nil
}

func (c *imapClient) fetchMessages(accountID string, mailboxID string, mailboxName string, uids []string) ([]Message, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	if responses, err := c.command("SELECT %s", quoteIMAPString(mailboxName)); err != nil {
		return nil, err
	} else if taggedStatus(responses) != "OK" {
		return nil, errors.New("select failed")
	}
	responses, err := c.command("UID FETCH %s (BODY.PEEK[])", strings.Join(uids, ","))
	if err != nil {
		return nil, err
	}
	var messages []Message
	for _, response := range responses {
		if len(response.literal) == 0 {
			continue
		}
		uid := parseUID(response.line)
		if uid == "" {
			continue
		}
		message, err := parseRawMessage(accountID, mailboxID, uid, response.literal)
		if err == nil {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func (c *imapClient) command(format string, args ...any) ([]imapResponse, error) {
	c.next++
	tag := fmt.Sprintf("A%03d", c.next)
	if _, err := fmt.Fprintf(c.conn, tag+" "+format+"\r\n", args...); err != nil {
		return nil, err
	}
	var responses []imapResponse
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return responses, err
		}
		line = strings.TrimRight(line, "\r\n")
		response := imapResponse{line: line}
		if size, ok := literalSize(line); ok {
			literal := make([]byte, size)
			if _, err := io.ReadFull(c.reader, literal); err != nil {
				return responses, err
			}
			response.literal = literal
		}
		responses = append(responses, response)
		if strings.HasPrefix(line, tag+" ") || strings.HasPrefix(line, tag+"\t") {
			return responses, nil
		}
	}
}

func taggedStatus(responses []imapResponse) string {
	if len(responses) == 0 {
		return ""
	}
	fields := strings.Fields(responses[len(responses)-1].line)
	if len(fields) < 2 {
		return ""
	}
	return strings.ToUpper(fields[1])
}

func literalSize(line string) (int, bool) {
	start := strings.LastIndex(line, "{")
	end := strings.LastIndex(line, "}")
	if start == -1 || end == -1 || end < start {
		return 0, false
	}
	size, err := strconv.Atoi(line[start+1 : end])
	return size, err == nil
}

func parseUID(line string) string {
	fields := strings.Fields(line)
	for i, field := range fields {
		if strings.EqualFold(strings.Trim(field, "()"), "UID") && i+1 < len(fields) {
			return strings.Trim(fields[i+1], ")")
		}
	}
	return ""
}

func parseLastQuoted(line string) string {
	end := strings.LastIndex(line, `"`)
	if end <= 0 {
		return ""
	}
	start := strings.LastIndex(line[:end], `"`)
	if start == -1 {
		return ""
	}
	return line[start+1 : end]
}

func mailboxFromIMAPName(name string) Mailbox {
	return Mailbox{
		ID:    mailboxID(name),
		Label: mailboxLabel(name),
		Kind:  mailboxKind(name),
	}
}

func mailboxID(name string) string {
	if strings.EqualFold(name, "INBOX") {
		return "inbox"
	}
	return "mbox_" + base64.RawURLEncoding.EncodeToString([]byte(name))
}

func decodeMailboxID(id string) (string, error) {
	if strings.EqualFold(id, "inbox") {
		return "INBOX", nil
	}
	if strings.HasPrefix(id, "mbox_") {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "mbox_"))
		return string(decoded), err
	}
	return id, nil
}

func mailboxLabel(name string) string {
	switch strings.ToLower(name) {
	case "inbox":
		return "받은편지함"
	case "sent":
		return "보낸편지함"
	case "drafts":
		return "임시보관함"
	case "archive":
		return "보관함"
	case "spam", "junk":
		return "스팸"
	case "trash":
		return "휴지통"
	default:
		parts := strings.Split(name, "/")
		return parts[len(parts)-1]
	}
}

func mailboxKind(name string) string {
	switch strings.ToLower(name) {
	case "inbox":
		return "inbox"
	case "sent":
		return "sent"
	case "drafts":
		return "drafts"
	case "archive":
		return "archive"
	case "spam", "junk":
		return "spam"
	case "trash":
		return "trash"
	default:
		return "folder"
	}
}

func messageID(mailboxID string, uid string) string {
	return "msg_" + base64.RawURLEncoding.EncodeToString([]byte(mailboxID+"\x00"+uid))
}

func decodeMessageID(id string) (string, string, error) {
	if !strings.HasPrefix(id, "msg_") {
		return "", "", errors.New("invalid message id")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "msg_"))
	if err != nil {
		return "", "", err
	}
	mailboxID, uid, ok := strings.Cut(string(decoded), "\x00")
	if !ok || mailboxID == "" || uid == "" {
		return "", "", errors.New("invalid message id")
	}
	return mailboxID, uid, nil
}

func parseRawMessage(accountID string, mailboxID string, uid string, raw []byte) (Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Message{}, err
	}
	header := msg.Header
	subject := decodeHeader(header.Get("Subject"))
	fromName, fromEmail := parseAddress(header.Get("From"))
	date := header.Get("Date")
	body, attachments := parseMessageBody(header, msg.Body)
	snippet := ""
	if len(body) > 0 {
		snippet = strings.TrimSpace(body[0])
		if len([]rune(snippet)) > 140 {
			snippet = string([]rune(snippet)[:140])
		}
	}

	return Message{
		MessageSummary: MessageSummary{
			ID:            messageID(mailboxID, uid),
			AccountID:     accountID,
			MailboxID:     mailboxID,
			Sender:        fromName,
			SenderEmail:   fromEmail,
			Initials:      firstInitial(fromName),
			Subject:       subject,
			Snippet:       snippet,
			Time:          shortMailTime(date),
			FullDate:      date,
			Unread:        false,
			HasAttachment: len(attachments) > 0,
		},
		Headers: MessageHead{
			From:    header.Get("From"),
			To:      header["To"],
			Cc:      header["Cc"],
			Date:    date,
			Subject: subject,
		},
		TextBody:    body,
		Attachments: attachments,
	}, nil
}

func parseMessageBody(header mail.Header, body io.Reader) ([]string, []Attachment) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		var text []string
		var attachments []Attachment
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				break
			}
			partHeader := mail.Header(part.Header)
			partText, partAttachments := parsePart(partHeader, part)
			text = append(text, partText...)
			attachments = append(attachments, partAttachments...)
		}
		return text, attachments
	}
	decoded := decodeTransfer(header.Get("Content-Transfer-Encoding"), body)
	return splitParagraphs(string(decoded)), nil
}

func parsePart(header mail.Header, body io.Reader) ([]string, []Attachment) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return parseMessageBody(header, body)
	}
	disposition, dispositionParams, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
	filename := dispositionParams["filename"]
	if filename == "" {
		filename = params["name"]
	}
	decoded := decodeTransfer(header.Get("Content-Transfer-Encoding"), body)
	if strings.EqualFold(disposition, "attachment") || filename != "" {
		return nil, []Attachment{{
			Name: decodeHeader(filename),
			Size: formatBytes(len(decoded)),
			Type: attachmentType(mediaType),
		}}
	}
	if strings.EqualFold(mediaType, "text/plain") || mediaType == "" {
		return splitParagraphs(string(decoded)), nil
	}
	return nil, nil
}

func decodeTransfer(encoding string, body io.Reader) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, body))
		if err == nil {
			return decoded
		}
	case "quoted-printable":
		decoded, err := io.ReadAll(quotedprintable.NewReader(body))
		if err == nil {
			return decoded
		}
	}
	decoded, _ := io.ReadAll(body)
	return decoded
}

func splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseAddress(value string) (string, string) {
	address, err := mail.ParseAddress(value)
	if err != nil {
		return decodeHeader(value), value
	}
	name := decodeHeader(address.Name)
	if name == "" {
		name = address.Address
	}
	return name, address.Address
}

func decodeHeader(value string) string {
	decoded, err := new(mime.WordDecoder).DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func shortMailTime(value string) string {
	parsed, err := mail.ParseDate(value)
	if err != nil {
		return value
	}
	now := time.Now()
	if parsed.Year() == now.Year() && parsed.YearDay() == now.YearDay() {
		return parsed.Format("15:04")
	}
	return parsed.Format("2006-01-02")
}

func attachmentType(mediaType string) string {
	if strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return "image"
	}
	if strings.EqualFold(mediaType, "application/pdf") {
		return "pdf"
	}
	return "file"
}

func formatBytes(size int) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
}
