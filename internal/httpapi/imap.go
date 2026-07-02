package httpapi

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

const imapCommandTimeout = 10 * time.Second

var remoteImageSrcPattern = regexp.MustCompile(`(?i)(<img\b[^>]*?)\s+src\s*=\s*("https?://[^"]*"|'https?://[^']*'|https?://[^\s>]+)`)

type deadlineReadWriteCloser interface {
	io.ReadWriteCloser
	SetDeadline(time.Time) error
}

type imapClient struct {
	conn   deadlineReadWriteCloser
	reader *bufio.Reader
	next   int
}

type imapResponse struct {
	line    string
	literal []byte
}

type AttachmentPayload struct {
	Name        string
	ContentType string
	Data        []byte
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
	_ = c.conn.SetDeadline(time.Now().Add(imapCommandTimeout))
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
	names, err := c.listMailboxNames()
	if err != nil {
		return nil, err
	}
	mailboxes := make([]Mailbox, 0, len(names))
	for _, name := range names {
		mailboxes = append(mailboxes, mailboxFromIMAPName(name))
	}
	sortMailboxes(mailboxes)
	if len(mailboxes) == 0 {
		mailboxes = append(mailboxes, mailboxFromIMAPName("INBOX"))
	}
	return mailboxes, nil
}

func (c *imapClient) listMailboxNames() ([]string, error) {
	responses, err := c.command(`LIST "" "*"`)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, response := range responses {
		if !strings.HasPrefix(response.line, "* LIST ") {
			continue
		}
		name, noselect := parseListMailboxName(response.line)
		if name == "" {
			continue
		}
		if skipMailboxListResponse(name, noselect) {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func (c *imapClient) appendSentMessage(raw string) error {
	names, err := c.listMailboxNames()
	if err != nil {
		return err
	}
	mailboxName := sentMailboxName(names)
	if mailboxName == "" {
		mailboxName = "Sent"
	}
	return c.appendMessage(mailboxName, raw)
}

func (c *imapClient) appendMessage(mailboxName string, raw string) error {
	c.next++
	tag := fmt.Sprintf("A%03d", c.next)
	if err := c.conn.SetDeadline(time.Now().Add(imapCommandTimeout)); err != nil {
		return err
	}
	literal := []byte(raw)
	if _, err := fmt.Fprintf(c.conn, "%s APPEND %s (\\Seen) {%d}\r\n", tag, quoteIMAPString(mailboxName), len(literal)); err != nil {
		return err
	}
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "+") {
			break
		}
		if strings.HasPrefix(line, tag+" ") || strings.HasPrefix(line, tag+"\t") {
			return errors.New("append failed")
		}
	}
	if _, err := c.conn.Write(literal); err != nil {
		return err
	}
	if _, err := c.conn.Write([]byte("\r\n")); err != nil {
		return err
	}
	responses, err := c.readUntilTag(tag)
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("append failed")
	}
	return nil
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
	sortMessagesNewestFirst(messages)
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
	message := messages[0]
	if message.Unread {
		if err := c.markMessageSeen(uid); err != nil {
			return Message{}, err
		}
		message.Unread = false
	}
	return message, nil
}

func (c *imapClient) messageAttachment(messageID string, attachmentID string) (AttachmentPayload, error) {
	mailboxID, uid, err := decodeMessageID(messageID)
	if err != nil {
		return AttachmentPayload{}, ErrNotFound
	}
	mailboxName, err := decodeMailboxID(mailboxID)
	if err != nil {
		return AttachmentPayload{}, ErrNotFound
	}
	if responses, err := c.command("SELECT %s", quoteIMAPString(mailboxName)); err != nil {
		return AttachmentPayload{}, err
	} else if taggedStatus(responses) != "OK" {
		return AttachmentPayload{}, ErrNotFound
	}
	responses, err := c.command("UID FETCH %s (BODY.PEEK[])", uid)
	if err != nil {
		return AttachmentPayload{}, err
	}
	for _, response := range responses {
		if len(response.literal) == 0 || parseUID(response.line) != uid {
			continue
		}
		attachment, err := extractAttachmentPayload(response.literal, attachmentID)
		if errors.Is(err, ErrNotFound) {
			return AttachmentPayload{}, ErrNotFound
		}
		return attachment, err
	}
	return AttachmentPayload{}, ErrNotFound
}

func (c *imapClient) markMessageSeen(uid string) error {
	responses, err := c.command("UID STORE %s +FLAGS.SILENT (\\Seen)", uid)
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("store failed")
	}
	return nil
}

func (c *imapClient) setMessageSeen(messageID string, seen bool) error {
	mailboxID, uid, err := decodeMessageID(messageID)
	if err != nil {
		return ErrNotFound
	}
	mailboxName, err := decodeMailboxID(mailboxID)
	if err != nil {
		return ErrNotFound
	}
	if responses, err := c.command("SELECT %s", quoteIMAPString(mailboxName)); err != nil {
		return err
	} else if taggedStatus(responses) != "OK" {
		return ErrNotFound
	}
	operation := "+FLAGS.SILENT"
	if !seen {
		operation = "-FLAGS.SILENT"
	}
	responses, err := c.command("UID STORE %s %s (\\Seen)", uid, operation)
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("store failed")
	}
	return nil
}

func (c *imapClient) setMessageFlagged(messageID string, flagged bool) error {
	mailboxID, uid, err := decodeMessageID(messageID)
	if err != nil {
		return ErrNotFound
	}
	mailboxName, err := decodeMailboxID(mailboxID)
	if err != nil {
		return ErrNotFound
	}
	if responses, err := c.command("SELECT %s", quoteIMAPString(mailboxName)); err != nil {
		return err
	} else if taggedStatus(responses) != "OK" {
		return ErrNotFound
	}
	operation := "+FLAGS.SILENT"
	if !flagged {
		operation = "-FLAGS.SILENT"
	}
	responses, err := c.command("UID STORE %s %s (\\Flagged)", uid, operation)
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("store failed")
	}
	return nil
}

func (c *imapClient) moveMessage(messageID string, targetMailboxID string) error {
	sourceMailboxID, uid, err := decodeMessageID(messageID)
	if err != nil {
		return ErrNotFound
	}
	sourceMailboxName, err := decodeMailboxID(sourceMailboxID)
	if err != nil {
		return ErrNotFound
	}
	targetMailboxName, err := decodeMailboxID(targetMailboxID)
	if err != nil {
		return ErrNotFound
	}
	if responses, err := c.command("SELECT %s", quoteIMAPString(sourceMailboxName)); err != nil {
		return err
	} else if taggedStatus(responses) != "OK" {
		return ErrNotFound
	}
	responses, err := c.command("UID MOVE %s %s", uid, quoteIMAPString(targetMailboxName))
	if err != nil {
		return err
	}
	if taggedStatus(responses) == "OK" {
		return nil
	}
	if err := c.copyThenDeleteMessage(uid, targetMailboxName); err != nil {
		return err
	}
	return nil
}

func (c *imapClient) copyThenDeleteMessage(uid string, targetMailboxName string) error {
	responses, err := c.command("UID COPY %s %s", uid, quoteIMAPString(targetMailboxName))
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("copy failed")
	}
	responses, err = c.command("UID STORE %s +FLAGS.SILENT (\\Deleted)", uid)
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("store failed")
	}
	responses, err = c.command("EXPUNGE")
	if err != nil {
		return err
	}
	if taggedStatus(responses) != "OK" {
		return errors.New("expunge failed")
	}
	return nil
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
	responses, err := c.command("UID FETCH %s (FLAGS BODY.PEEK[])", strings.Join(uids, ","))
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
			message.Unread = !responseSeen(response.line)
			message.Flagged = responseFlagged(response.line)
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func (c *imapClient) command(format string, args ...any) ([]imapResponse, error) {
	c.next++
	tag := fmt.Sprintf("A%03d", c.next)
	if err := c.conn.SetDeadline(time.Now().Add(imapCommandTimeout)); err != nil {
		return nil, err
	}
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

func (c *imapClient) readUntilTag(tag string) ([]imapResponse, error) {
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

func responseSeen(line string) bool {
	return strings.Contains(strings.ToUpper(line), `\SEEN`)
}

func responseFlagged(line string) bool {
	return strings.Contains(strings.ToUpper(line), `\FLAGGED`)
}

func skipMailboxListResponse(name string, noselect bool) bool {
	return noselect || name == "."
}

func sortMailboxes(mailboxes []Mailbox) {
	sort.SliceStable(mailboxes, func(i, j int) bool {
		leftOrder := mailboxKindOrder(mailboxes[i].Kind)
		rightOrder := mailboxKindOrder(mailboxes[j].Kind)
		if leftOrder != rightOrder {
			return leftOrder < rightOrder
		}
		return strings.ToLower(mailboxes[i].Label) < strings.ToLower(mailboxes[j].Label)
	})
}

func mailboxKindOrder(kind string) int {
	switch kind {
	case "inbox":
		return 0
	case "sent":
		return 1
	case "drafts":
		return 2
	case "archive":
		return 3
	case "spam":
		return 4
	case "trash":
		return 5
	default:
		return 100
	}
}

func sentMailboxName(names []string) string {
	for _, name := range names {
		if isSentMailboxName(name) {
			return name
		}
	}
	return ""
}

func isSentMailboxName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "sent", "sent mail", "sent messages":
		return true
	}
	return strings.HasSuffix(normalized, "/sent") || strings.HasSuffix(normalized, ".sent")
}

func parseListMailboxName(line string) (string, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "* LIST "))
	if !strings.HasPrefix(rest, "(") {
		return "", false
	}
	flagsEnd := strings.Index(rest, ")")
	if flagsEnd == -1 {
		return "", false
	}
	flags := strings.ToUpper(rest[:flagsEnd+1])
	noselect := strings.Contains(flags, `\NOSELECT`)
	rest = strings.TrimSpace(rest[flagsEnd+1:])
	_, rest, ok := parseIMAPListToken(rest)
	if !ok {
		return "", noselect
	}
	name, _, ok := parseIMAPListToken(rest)
	if !ok {
		return "", noselect
	}
	return name, noselect
}

func parseIMAPListToken(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	if value[0] != '"' {
		token, rest, _ := strings.Cut(value, " ")
		if strings.EqualFold(token, "NIL") {
			return "", rest, true
		}
		return token, rest, true
	}
	var builder strings.Builder
	escaped := false
	for i := 1; i < len(value); i++ {
		character := value[i]
		if escaped {
			builder.WriteByte(character)
			escaped = false
			continue
		}
		if character == '\\' {
			escaped = true
			continue
		}
		if character == '"' {
			return builder.String(), value[i+1:], true
		}
		builder.WriteByte(character)
	}
	return "", "", false
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
		return decodeIMAPModifiedUTF7(parts[len(parts)-1])
	}
}

func decodeIMAPModifiedUTF7(value string) string {
	var builder strings.Builder
	for i := 0; i < len(value); {
		if value[i] != '&' {
			builder.WriteByte(value[i])
			i++
			continue
		}
		end := strings.IndexByte(value[i+1:], '-')
		if end == -1 {
			builder.WriteByte(value[i])
			i++
			continue
		}
		encoded := value[i+1 : i+1+end]
		if encoded == "" {
			builder.WriteByte('&')
			i += 2
			continue
		}
		decoded, ok := decodeModifiedUTF7Segment(encoded)
		if !ok {
			builder.WriteString(value[i : i+end+2])
		} else {
			builder.WriteString(decoded)
		}
		i += end + 2
	}
	return builder.String()
}

func decodeModifiedUTF7Segment(encoded string) (string, bool) {
	base64Value := strings.ReplaceAll(encoded, ",", "/")
	if remainder := len(base64Value) % 4; remainder != 0 {
		base64Value += strings.Repeat("=", 4-remainder)
	}
	decoded, err := base64.StdEncoding.DecodeString(base64Value)
	if err != nil || len(decoded)%2 != 0 {
		return "", false
	}
	words := make([]uint16, 0, len(decoded)/2)
	for i := 0; i < len(decoded); i += 2 {
		words = append(words, uint16(decoded[i])<<8|uint16(decoded[i+1]))
	}
	return string(utf16.Decode(words)), true
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
	body, htmlBody, attachments, remoteImagesBlocked := parseMessageBody(header, msg.Body)
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
		RemoteImagesBlocked: remoteImagesBlocked,
		TextBody:            body,
		HTMLBody:            htmlBody,
		Attachments:         attachments,
	}, nil
}

func sortMessagesNewestFirst(messages []Message) {
	sort.SliceStable(messages, func(i, j int) bool {
		leftDate, leftOK := parsedMailDate(messages[i].FullDate)
		rightDate, rightOK := parsedMailDate(messages[j].FullDate)
		if leftOK && rightOK && !leftDate.Equal(rightDate) {
			return leftDate.After(rightDate)
		}
		if leftOK != rightOK {
			return leftOK
		}
		return messageUID(messages[i].ID) > messageUID(messages[j].ID)
	})
}

func parsedMailDate(value string) (time.Time, bool) {
	parsed, err := mail.ParseDate(value)
	return parsed, err == nil
}

func messageUID(id string) int {
	_, uid, err := decodeMessageID(id)
	if err != nil {
		return 0
	}
	parsed, err := strconv.Atoi(uid)
	if err != nil {
		return 0
	}
	return parsed
}

func parseMessageBody(header mail.Header, body io.Reader) ([]string, string, []Attachment, bool) {
	counter := 0
	return parseMessageBodyWithCounter(header, body, &counter)
}

func parseMessageBodyWithCounter(header mail.Header, body io.Reader, counter *int) ([]string, string, []Attachment, bool) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		var text []string
		var html []string
		var attachments []Attachment
		remoteImagesBlocked := false
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				break
			}
			partHeader := mail.Header(part.Header)
			partText, partHTML, partAttachments, partRemoteImagesBlocked := parsePart(partHeader, part, counter)
			text = append(text, partText...)
			if partHTML != "" {
				html = append(html, partHTML)
			}
			attachments = append(attachments, partAttachments...)
			remoteImagesBlocked = remoteImagesBlocked || partRemoteImagesBlocked
		}
		return text, strings.Join(html, "\n"), attachments, remoteImagesBlocked
	}
	decoded := decodePartBody(header, body)
	if strings.EqualFold(mediaType, "text/html") {
		html, remoteImagesBlocked := sanitizeMailHTML(string(decoded))
		return nil, html, nil, remoteImagesBlocked
	}
	return splitParagraphs(string(decoded)), "", nil, false
}

func parsePart(header mail.Header, body io.Reader, counter *int) ([]string, string, []Attachment, bool) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return parseMessageBodyWithCounter(header, body, counter)
	}
	disposition, dispositionParams, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
	filename := dispositionParams["filename"]
	if filename == "" {
		filename = params["name"]
	}
	decoded := decodePartBody(header, body)
	if strings.EqualFold(disposition, "attachment") || filename != "" {
		id := fmt.Sprintf("att_%d", *counter)
		*counter++
		return nil, "", []Attachment{{
			ID:   id,
			Name: decodeHeader(filename),
			Size: formatBytes(len(decoded)),
			Type: attachmentType(mediaType),
		}}, false
	}
	if strings.EqualFold(mediaType, "text/plain") || mediaType == "" {
		return splitParagraphs(string(decoded)), "", nil, false
	}
	if strings.EqualFold(mediaType, "text/html") {
		html, remoteImagesBlocked := sanitizeMailHTML(string(decoded))
		return nil, html, nil, remoteImagesBlocked
	}
	return nil, "", nil, false
}

func extractAttachmentPayload(raw []byte, attachmentID string) (AttachmentPayload, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return AttachmentPayload{}, err
	}
	counter := 0
	return findAttachmentPayload(msg.Header, msg.Body, attachmentID, &counter)
}

func findAttachmentPayload(header mail.Header, body io.Reader, attachmentID string, counter *int) (AttachmentPayload, error) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return AttachmentPayload{}, err
			}
			attachment, err := findAttachmentPayload(mail.Header(part.Header), part, attachmentID, counter)
			if err == nil {
				return attachment, nil
			}
			if !errors.Is(err, ErrNotFound) {
				return AttachmentPayload{}, err
			}
		}
		return AttachmentPayload{}, ErrNotFound
	}
	disposition, dispositionParams, _ := mime.ParseMediaType(header.Get("Content-Disposition"))
	filename := dispositionParams["filename"]
	if filename == "" {
		filename = params["name"]
	}
	if !strings.EqualFold(disposition, "attachment") && filename == "" {
		_, _ = io.Copy(io.Discard, body)
		return AttachmentPayload{}, ErrNotFound
	}
	id := fmt.Sprintf("att_%d", *counter)
	*counter++
	decoded := decodePartBody(header, body)
	if id != attachmentID {
		return AttachmentPayload{}, ErrNotFound
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return AttachmentPayload{
		Name:        decodeHeader(filename),
		ContentType: mediaType,
		Data:        decoded,
	}, nil
}

func decodePartBody(header mail.Header, body io.Reader) []byte {
	decoded := decodeTransfer(header.Get("Content-Transfer-Encoding"), body)
	_, params, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		return decoded
	}
	return decodeCharset(params["charset"], decoded)
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

func decodeCharset(name string, value []byte) []byte {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "utf-8") || strings.EqualFold(name, "us-ascii") {
		return value
	}
	encoding, err := htmlindex.Get(name)
	if err != nil || encoding == nil {
		return value
	}
	decoded, err := io.ReadAll(transform.NewReader(bytes.NewReader(value), encoding.NewDecoder()))
	if err != nil {
		return value
	}
	return decoded
}

func sanitizeMailHTML(value string) (string, bool) {
	remoteImagesBlocked := hasRemoteImage(value)
	value = blockRemoteImageSources(value)
	policy := bluemonday.NewPolicy()
	policy.AllowStandardURLs()
	policy.AllowElements("a", "b", "blockquote", "br", "code", "div", "em", "hr", "i", "img", "li", "ol", "p", "pre", "span", "strong", "table", "tbody", "td", "th", "thead", "tr", "u", "ul")
	policy.AllowAttrs("href").OnElements("a")
	policy.AllowAttrs("alt", "data-joomail-remote-src", "height", "width").OnElements("img")
	return policy.Sanitize(value), remoteImagesBlocked
}

func hasRemoteImage(value string) bool {
	return remoteImageSrcPattern.MatchString(value)
}

func blockRemoteImageSources(value string) string {
	return remoteImageSrcPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := remoteImageSrcPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		url := strings.Trim(parts[2], `"'`)
		return parts[1] + ` data-joomail-remote-src="` + html.EscapeString(url) + `"`
	})
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
	decoder := mime.WordDecoder{CharsetReader: charsetReader}
	decoded, err := decoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func charsetReader(name string, input io.Reader) (io.Reader, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "utf-8") || strings.EqualFold(name, "us-ascii") {
		return input, nil
	}
	encoding, err := htmlindex.Get(name)
	if err != nil || encoding == nil {
		return input, nil
	}
	return transform.NewReader(input, encoding.NewDecoder()), nil
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
