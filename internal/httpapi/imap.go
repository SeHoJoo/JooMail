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
	"net/url"
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

const (
	imapCommandTimeout  = 10 * time.Second
	messageSummaryLimit = 50
)

var ErrNotFound = errors.New("not found")

var remoteImageSrcPattern = regexp.MustCompile(`(?i)(<img\b[^>]*?)\s+src\s*=\s*("https?://[^"]*"|'https?://[^']*'|https?://[^\s>]+)`)
var cidImageSrcPattern = regexp.MustCompile(`(?i)(<img\b[^>]*?)\s+src\s*=\s*("cid:[^"]+"|'cid:[^']+'|cid:[^\s>]+)`)
var dataImageSrcPattern = regexp.MustCompile(`(?i)^data:image/(gif|jpeg|png|webp);base64,[a-z0-9+/]+=*$`)
var messageIDPattern = regexp.MustCompile(`<([^>]+)>`)

type inlineImage struct {
	mediaType string
	data      []byte
}

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

type mailboxListEntry struct {
	name      string
	delimiter string
	noselect  bool
}

type messageSearchScope string

const (
	messageSearchScopeMailbox messageSearchScope = "mailbox"
	messageSearchScopeAccount messageSearchScope = "account"
)

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
	entries, err := c.listMailboxEntries()
	if err != nil {
		return nil, err
	}
	mailboxes := buildMailboxTree(entries)
	if len(mailboxes) == 0 {
		mailboxes = append(mailboxes, mailboxFromIMAPName("INBOX", "", false))
	}
	return mailboxes, nil
}

func (c *imapClient) listMailboxNames() ([]string, error) {
	entries, err := c.listMailboxEntries()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.noselect {
			continue
		}
		names = append(names, entry.name)
	}
	return names, nil
}

func (c *imapClient) listMailboxEntries() ([]mailboxListEntry, error) {
	responses, err := c.command(`LIST "" "*"`)
	if err != nil {
		return nil, err
	}
	var entries []mailboxListEntry
	for _, response := range responses {
		if !strings.HasPrefix(response.line, "* LIST ") {
			continue
		}
		name, delimiter, noselect := parseListMailboxName(response.line)
		if name == "" {
			continue
		}
		if skipMailboxListResponse(name) {
			continue
		}
		entries = append(entries, mailboxListEntry{name: name, delimiter: delimiter, noselect: noselect})
	}
	return entries, nil
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
	return c.appendMessage(mailboxName, raw, "(\\Seen)")
}

func (c *imapClient) appendDraftMessage(raw string) error {
	names, err := c.listMailboxNames()
	if err != nil {
		return err
	}
	mailboxName := draftsMailboxName(names)
	if mailboxName == "" {
		mailboxName = "Drafts"
	}
	return c.appendMessage(mailboxName, raw, "(\\Draft)")
}

func (c *imapClient) appendMessage(mailboxName string, raw string, flags string) error {
	c.next++
	tag := fmt.Sprintf("A%03d", c.next)
	if err := c.conn.SetDeadline(time.Now().Add(imapCommandTimeout)); err != nil {
		return err
	}
	literal := []byte(raw)
	if _, err := fmt.Fprintf(c.conn, "%s APPEND %s %s {%d}\r\n", tag, quoteIMAPString(mailboxName), flags, len(literal)); err != nil {
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

func (c *imapClient) messageSummaries(accountID string, mailboxID string, query string, scope messageSearchScope) ([]MessageSummary, error) {
	if mailboxID == "starred" {
		return c.starredMessageSummaries(accountID, query)
	}
	mailboxName, err := decodeMailboxID(mailboxID)
	if err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	if scope == messageSearchScopeAccount && query != "" {
		return c.accountMessageSummaries(accountID, query)
	}
	uids, err := c.searchMailbox(mailboxName, query)
	if err != nil {
		return nil, err
	}
	if len(uids) > messageSummaryLimit {
		uids = uids[:messageSummaryLimit]
	}
	messages, err := c.fetchMessages(accountID, mailboxID, mailboxName, uids)
	if err != nil {
		return nil, err
	}
	sortMessagesNewestFirst(messages)
	summaries := make([]MessageSummary, 0, len(messages))
	for _, message := range messages {
		summaries = append(summaries, message.MessageSummary)
	}
	return summaries, nil
}

func (c *imapClient) starredMessageSummaries(accountID string, query string) ([]MessageSummary, error) {
	names, err := c.listMailboxNames()
	if err != nil {
		return nil, err
	}
	var messages []Message
	for _, name := range names {
		uids, err := c.searchMailboxCriteria(name, flaggedSearchCriteria(query))
		if err != nil {
			return nil, err
		}
		if len(uids) > messageSummaryLimit {
			uids = uids[:messageSummaryLimit]
		}
		items, err := c.fetchMessages(accountID, mailboxID(name), name, uids)
		if err != nil {
			return nil, err
		}
		messages = append(messages, items...)
	}
	sortMessagesNewestFirst(messages)
	if len(messages) > messageSummaryLimit {
		messages = messages[:messageSummaryLimit]
	}
	out := make([]MessageSummary, 0, len(messages))
	for _, message := range messages {
		out = append(out, message.MessageSummary)
	}
	return out, nil
}

func (c *imapClient) accountMessageSummaries(accountID string, query string) ([]MessageSummary, error) {
	mailboxNames, err := c.listMailboxNames()
	if err != nil {
		return nil, err
	}
	var messages []Message
	for _, mailboxName := range mailboxNames {
		mailboxID := mailboxID(mailboxName)
		uids, err := c.searchMailbox(mailboxName, query)
		if err != nil {
			return nil, err
		}
		if len(uids) > messageSummaryLimit {
			uids = uids[:messageSummaryLimit]
		}
		mailboxMessages, err := c.fetchMessages(accountID, mailboxID, mailboxName, uids)
		if err != nil {
			return nil, err
		}
		messages = append(messages, mailboxMessages...)
	}
	sortMessagesNewestFirst(messages)
	if len(messages) > messageSummaryLimit {
		messages = messages[:messageSummaryLimit]
	}
	summaries := make([]MessageSummary, 0, len(messages))
	for _, message := range messages {
		summaries = append(summaries, message.MessageSummary)
	}
	return summaries, nil
}

func (c *imapClient) message(accountID string, messageID string) (Message, error) {
	_, mailboxID, expectedUIDValidity, uid, _, err := decodeMessageReference(messageID)
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
	if expectedUIDValidity != "" && expectedUIDValidity != messageUIDValidity(messages[0].ID) {
		return Message{}, ErrNotFound
	}
	return messages[0], nil
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
	capabilities, err := c.capabilities()
	if err != nil {
		return err
	}
	if capabilities["MOVE"] {
		responses, err := c.command("UID MOVE %s %s", uid, quoteIMAPString(targetMailboxName))
		if err != nil {
			return err
		}
		if taggedStatus(responses) != "OK" {
			return errors.New("move failed")
		}
		return nil
	}
	return c.copyThenDeleteMessage(uid, targetMailboxName, capabilities["UIDPLUS"])
}

func (c *imapClient) capabilities() (map[string]bool, error) {
	responses, err := c.command("CAPABILITY")
	if err != nil {
		return nil, err
	}
	if taggedStatus(responses) != "OK" {
		return nil, errors.New("capability failed")
	}
	capabilities := make(map[string]bool)
	for _, response := range responses {
		if !strings.HasPrefix(strings.ToUpper(response.line), "* CAPABILITY ") {
			continue
		}
		for _, capability := range strings.Fields(strings.TrimSpace(response.line[len("* CAPABILITY "):])) {
			capabilities[strings.ToUpper(capability)] = true
		}
	}
	return capabilities, nil
}

func (c *imapClient) copyThenDeleteMessage(uid string, targetMailboxName string, uidPlus bool) error {
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
	if uidPlus {
		_, _ = c.command("UID EXPUNGE %s", uid)
	}
	return nil
}

func (c *imapClient) searchMailbox(mailboxName string, query string) ([]string, error) {
	return c.searchMailboxCriteria(mailboxName, searchCriteria(query))
}

func (c *imapClient) searchMailboxCriteria(mailboxName string, criteria string) ([]string, error) {
	if responses, err := c.command("SELECT %s", quoteIMAPString(mailboxName)); err != nil {
		return nil, err
	} else if taggedStatus(responses) != "OK" {
		return nil, errors.New("select failed")
	}
	responses, err := c.command("UID SEARCH %s", criteria)
	if err != nil {
		return nil, err
	}
	if taggedStatus(responses) != "OK" {
		fallbackCriteria, ok := searchCriteriaWithoutCharset(criteria)
		if !ok {
			return nil, errors.New("search failed")
		}
		responses, err = c.command("UID SEARCH %s", fallbackCriteria)
		if err != nil {
			return nil, err
		}
		if taggedStatus(responses) != "OK" {
			return nil, errors.New("search failed")
		}
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

func flaggedSearchCriteria(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "FLAGGED NOT DELETED"
	}
	return "FLAGGED NOT DELETED TEXT " + quoteIMAPString(query)
}

func searchCriteria(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return "NOT DELETED"
	}
	criterion := "NOT DELETED TEXT " + quoteIMAPString(query)
	if isASCII(query) {
		return criterion
	}
	return "CHARSET UTF-8 " + criterion
}

func searchCriteriaWithoutCharset(criteria string) (string, bool) {
	fallback, ok := strings.CutPrefix(criteria, "CHARSET UTF-8 ")
	if !ok {
		return "", false
	}
	return fallback, true
}

func isASCII(value string) bool {
	for _, r := range value {
		if r > 127 {
			return false
		}
	}
	return true
}

func parseMessageSearchScope(value string) (messageSearchScope, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(messageSearchScopeMailbox):
		return messageSearchScopeMailbox, nil
	case string(messageSearchScopeAccount):
		return messageSearchScopeAccount, nil
	default:
		return "", errors.New("invalid search scope")
	}
}

func (c *imapClient) withUnreadCounts(mailboxes []Mailbox) []Mailbox {
	counts := make(map[string]int)
	for _, mailbox := range flattenSelectableMailboxes(mailboxes) {
		name, err := decodeMailboxID(mailbox.ID)
		if err != nil {
			continue
		}
		count, err := c.mailboxUnreadCount(name)
		if err != nil {
			continue
		}
		counts[mailbox.ID] = count
	}
	mailboxes = applyUnreadCounts(mailboxes, counts)
	starred, err := c.starredUnreadCount(flattenSelectableMailboxes(mailboxes))
	if err == nil {
		mailboxes = addStarredMailbox(mailboxes, starred)
	}
	return mailboxes
}

func (c *imapClient) starredUnreadCount(mailboxes []Mailbox) (int, error) {
	total := 0
	for _, mailbox := range mailboxes {
		name, err := decodeMailboxID(mailbox.ID)
		if err != nil {
			continue
		}
		uids, err := c.searchMailboxCriteria(name, "FLAGGED UNSEEN NOT DELETED")
		if err != nil {
			return 0, err
		}
		total += len(uids)
	}
	return total, nil
}

func addStarredMailbox(mailboxes []Mailbox, unread int) []Mailbox {
	starred := Mailbox{ID: "starred", Label: "중요 표시", Kind: "starred", Selectable: true, Unread: unread}
	for i, mailbox := range mailboxes {
		if mailbox.Kind == "inbox" {
			return append(append(mailboxes[:i+1:i+1], starred), mailboxes[i+1:]...)
		}
	}
	return append([]Mailbox{starred}, mailboxes...)
}

func (c *imapClient) mailboxUnreadCount(mailboxName string) (int, error) {
	responses, err := c.command("STATUS %s (UNSEEN)", quoteIMAPString(mailboxName))
	if err != nil {
		return 0, err
	}
	if taggedStatus(responses) != "OK" {
		return 0, errors.New("status failed")
	}
	for _, response := range responses {
		if count, ok := parseUnreadStatus(response.line); ok {
			return count, nil
		}
	}
	return 0, nil
}

func flattenSelectableMailboxes(mailboxes []Mailbox) []Mailbox {
	var out []Mailbox
	for _, mailbox := range mailboxes {
		if mailbox.Selectable {
			out = append(out, mailbox)
		}
		out = append(out, flattenSelectableMailboxes(mailbox.Children)...)
	}
	return out
}

func applyUnreadCounts(mailboxes []Mailbox, counts map[string]int) []Mailbox {
	out := make([]Mailbox, len(mailboxes))
	for i, mailbox := range mailboxes {
		mailbox.Unread = counts[mailbox.ID]
		mailbox.Children = applyUnreadCounts(mailbox.Children, counts)
		out[i] = mailbox
	}
	return out
}

func parseUnreadStatus(line string) (int, bool) {
	upper := strings.ToUpper(line)
	index := strings.Index(upper, "UNSEEN")
	if !strings.HasPrefix(upper, "* STATUS ") || index == -1 {
		return 0, false
	}
	fields := strings.Fields(line[index:])
	if len(fields) < 2 {
		return 0, false
	}
	count, err := strconv.Atoi(strings.Trim(fields[1], "()"))
	return count, err == nil
}

func (c *imapClient) fetchMessages(accountID string, mailboxID string, mailboxName string, uids []string) ([]Message, error) {
	if len(uids) == 0 {
		return nil, nil
	}
	selectResponses, err := c.command("SELECT %s", quoteIMAPString(mailboxName))
	if err != nil {
		return nil, err
	} else if taggedStatus(selectResponses) != "OK" {
		return nil, errors.New("select failed")
	}
	uidValidity := selectUIDValidity(selectResponses)
	if uidValidity == "" {
		uidValidity = "0"
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
			message.ID = messageID(accountID, mailboxID, uidValidity, uid)
			message.Unread = !responseSeen(response.line)
			message.Flagged = responseFlagged(response.line)
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func selectUIDValidity(responses []imapResponse) string {
	for _, response := range responses {
		fields := strings.Fields(strings.Trim(response.line, "()"))
		for i, field := range fields {
			if strings.EqualFold(field, "UIDVALIDITY") && i+1 < len(fields) {
				return strings.Trim(fields[i+1], ")")
			}
		}
	}
	return ""
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
	rawSize := strings.TrimSuffix(line[start+1:end], "+")
	size, err := strconv.Atoi(rawSize)
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

func skipMailboxListResponse(name string) bool {
	return name == "."
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
	for i := range mailboxes {
		sortMailboxes(mailboxes[i].Children)
	}
}

func mailboxKindOrder(kind string) int {
	switch kind {
	case "inbox":
		return 0
	case "starred":
		return 1
	case "sent":
		return 2
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

func draftsMailboxName(names []string) string {
	for _, name := range names {
		if isDraftsMailboxName(name) {
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

func isDraftsMailboxName(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "draft", "drafts":
		return true
	}
	return strings.HasSuffix(normalized, "/drafts") || strings.HasSuffix(normalized, ".drafts") || strings.HasSuffix(normalized, "/draft") || strings.HasSuffix(normalized, ".draft")
}

func parseListMailboxName(line string) (string, string, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "* LIST "))
	if !strings.HasPrefix(rest, "(") {
		return "", "", false
	}
	flagsEnd := strings.Index(rest, ")")
	if flagsEnd == -1 {
		return "", "", false
	}
	flags := strings.ToUpper(rest[:flagsEnd+1])
	noselect := strings.Contains(flags, `\NOSELECT`)
	rest = strings.TrimSpace(rest[flagsEnd+1:])
	delimiter, rest, ok := parseIMAPListToken(rest)
	if !ok {
		return "", "", noselect
	}
	name, _, ok := parseIMAPListToken(rest)
	if !ok {
		return "", "", noselect
	}
	return name, delimiter, noselect
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

func buildMailboxTree(entries []mailboxListEntry) []Mailbox {
	nodes := make(map[string]Mailbox, len(entries))
	entryByName := make(map[string]mailboxListEntry, len(entries))
	childrenByParent := make(map[string][]string)
	childNames := make(map[string]bool)

	for _, entry := range entries {
		nodes[entry.name] = mailboxFromIMAPName(entry.name, "", entry.noselect)
		entryByName[entry.name] = entry
	}

	for _, entry := range entries {
		parentName := parentMailboxName(entry.name, entry.delimiter)
		if parentName == "" {
			continue
		}
		if _, ok := nodes[parentName]; ok {
			childrenByParent[parentName] = append(childrenByParent[parentName], entry.name)
			childNames[entry.name] = true
		}
	}

	var materialize func(string) Mailbox
	materialize = func(name string) Mailbox {
		mailbox := nodes[name]
		if childNames[name] {
			entry := entryByName[name]
			mailbox.Label = mailboxLabel(name, entry.delimiter)
		}
		for _, childName := range childrenByParent[name] {
			mailbox.Children = append(mailbox.Children, materialize(childName))
		}
		sortMailboxes(mailbox.Children)
		return mailbox
	}

	var mailboxes []Mailbox
	for _, entry := range entries {
		if childNames[entry.name] {
			continue
		}
		mailboxes = append(mailboxes, materialize(entry.name))
	}
	sortMailboxes(mailboxes)
	return mailboxes
}

func parentMailboxName(name string, delimiter string) string {
	if delimiter == "" {
		return ""
	}
	index := strings.LastIndex(name, delimiter)
	if index <= 0 {
		return ""
	}
	return name[:index]
}

func mailboxFromIMAPName(name string, delimiter string, noselect bool) Mailbox {
	return Mailbox{
		ID:         mailboxID(name),
		Label:      mailboxLabel(name, delimiter),
		Kind:       mailboxKind(name),
		Selectable: !noselect,
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

func mailboxLabel(name string, delimiter string) string {
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
		if delimiter != "" {
			parts := strings.Split(name, delimiter)
			return decodeIMAPModifiedUTF7(parts[len(parts)-1])
		}
		return decodeIMAPModifiedUTF7(name)
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

func messageID(parts ...string) string {
	if len(parts) == 2 { // test and legacy compatibility helper
		return "msg_" + base64.RawURLEncoding.EncodeToString([]byte(parts[0]+"\x00"+parts[1]))
	}
	if len(parts) != 4 {
		return ""
	}
	return "msg2_" + base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "\x00")))
}

func decodeMessageID(id string) (string, string, error) {
	if strings.HasPrefix(id, "msg2_") {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "msg2_"))
		if err != nil {
			return "", "", err
		}
		parts := strings.Split(string(decoded), "\x00")
		if len(parts) != 4 || parts[1] == "" || parts[3] == "" {
			return "", "", errors.New("invalid message id")
		}
		return parts[1], parts[3], nil
	}
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

func decodeMessageReference(id string) (accountID string, mailboxID string, uidValidity string, uid string, legacy bool, err error) {
	if !strings.HasPrefix(id, "msg2_") {
		mailboxID, uid, err = decodeMessageID(id)
		return "", mailboxID, "", uid, true, err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(id, "msg2_"))
	if err != nil {
		return "", "", "", "", false, err
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return "", "", "", "", false, errors.New("invalid message id")
	}
	return parts[0], parts[1], parts[2], parts[3], false, nil
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
	messageIDHeader := normalizeMessageID(header.Get("Message-ID"))
	inReplyTo := normalizeMessageID(header.Get("In-Reply-To"))
	references := parseMessageIDList(header.Get("References"))
	threadID := threadIDFromHeaders(messageIDHeader, inReplyTo, references)
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
			ID:            messageID(accountID, mailboxID, "0", uid),
			AccountID:     accountID,
			MailboxID:     mailboxID,
			ThreadID:      threadID,
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
			From:       header.Get("From"),
			To:         parseAddressListHeaders(header["To"]),
			Cc:         parseAddressListHeaders(header["Cc"]),
			Date:       date,
			Subject:    subject,
			MessageID:  messageIDHeader,
			InReplyTo:  inReplyTo,
			References: references,
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

func messageUIDValidity(id string) string {
	_, _, uidValidity, _, _, err := decodeMessageReference(id)
	if err != nil {
		return ""
	}
	return uidValidity
}

func parseMessageBody(header mail.Header, body io.Reader) ([]string, string, []Attachment, bool) {
	counter := 0
	return parseMessageBodyWithCounter(header, body, &counter)
}

func parseMessageBodyWithCounter(header mail.Header, body io.Reader, counter *int) ([]string, string, []Attachment, bool) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		if strings.EqualFold(mediaType, "multipart/related") {
			return parseRelatedMessageBody(body, params["boundary"], counter)
		}
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

func parseRelatedMessageBody(body io.Reader, boundary string, counter *int) ([]string, string, []Attachment, bool) {
	reader := multipart.NewReader(body, boundary)
	var text []string
	var htmlParts []string
	var attachments []Attachment
	inlineImages := map[string]inlineImage{}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		partText, partHTML, partAttachments := parseRelatedPart(mail.Header(part.Header), part, counter, inlineImages)
		text = append(text, partText...)
		if partHTML != "" {
			htmlParts = append(htmlParts, partHTML)
		}
		attachments = append(attachments, partAttachments...)
	}
	htmlBody := resolveCIDImageSources(strings.Join(htmlParts, "\n"), inlineImages)
	htmlBody, remoteImagesBlocked := sanitizeMailHTML(htmlBody)
	return text, htmlBody, attachments, remoteImagesBlocked
}

func parseRelatedPart(header mail.Header, body io.Reader, counter *int, inlineImages map[string]inlineImage) ([]string, string, []Attachment) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		reader := multipart.NewReader(body, params["boundary"])
		var text []string
		var html []string
		var attachments []Attachment
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				break
			}
			partText, partHTML, partAttachments := parseRelatedPart(mail.Header(part.Header), part, counter, inlineImages)
			text = append(text, partText...)
			if partHTML != "" {
				html = append(html, partHTML)
			}
			attachments = append(attachments, partAttachments...)
		}
		return text, strings.Join(html, "\n"), attachments
	}
	disposition, filename := attachmentDispositionAndFilename(header, params)
	decoded := decodePartBody(header, body)
	if isInlineCIDImage(header, mediaType, disposition) {
		inlineImages[normalizeContentID(header.Get("Content-ID"))] = inlineImage{mediaType: mediaType, data: decoded}
		return nil, "", nil
	}
	if strings.EqualFold(disposition, "attachment") || filename != "" {
		id := fmt.Sprintf("att_%d", *counter)
		*counter++
		return nil, "", []Attachment{{
			ID:   id,
			Name: decodeHeader(filename),
			Size: formatBytes(len(decoded)),
			Type: attachmentType(mediaType),
		}}
	}
	if strings.EqualFold(mediaType, "text/plain") || mediaType == "" {
		return splitParagraphs(string(decoded)), "", nil
	}
	if strings.EqualFold(mediaType, "text/html") {
		return nil, string(decoded), nil
	}
	return nil, "", nil
}

func parsePart(header mail.Header, body io.Reader, counter *int) ([]string, string, []Attachment, bool) {
	contentType := header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err == nil && strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
		return parseMessageBodyWithCounter(header, body, counter)
	}
	disposition, filename := attachmentDispositionAndFilename(header, params)
	decoded := decodePartBody(header, body)
	if isInlineCIDImage(header, mediaType, disposition) {
		return nil, "", nil, false
	}
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
	disposition, filename := attachmentDispositionAndFilename(header, params)
	if !strings.EqualFold(disposition, "attachment") && filename == "" {
		_, _ = io.Copy(io.Discard, body)
		return AttachmentPayload{}, ErrNotFound
	}
	if isInlineCIDImage(header, mediaType, disposition) {
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

func attachmentDispositionAndFilename(header mail.Header, contentTypeParams map[string]string) (string, string) {
	rawDisposition := strings.TrimSpace(header.Get("Content-Disposition"))
	disposition, dispositionParams, err := mime.ParseMediaType(rawDisposition)
	if err != nil && strings.HasPrefix(strings.ToLower(rawDisposition), "attachment") {
		disposition = "attachment"
	}
	filename := dispositionParams["filename"]
	if filename == "" {
		filename = contentTypeParams["name"]
	}
	if strings.EqualFold(disposition, "attachment") && filename == "" {
		filename = "attachment"
	}
	return disposition, filename
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
	policy.AllowElements("a", "b", "big", "blockquote", "br", "center", "code", "del", "div", "em", "font", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "i", "img", "ins", "li", "ol", "p", "pre", "s", "small", "span", "strike", "strong", "sub", "sup", "table", "tbody", "td", "tfoot", "th", "thead", "tr", "u", "ul")
	policy.AllowAttrs("href", "name", "target", "title").OnElements("a")
	policy.AllowAttrs("alt", "data-joomail-remote-src", "height", "width").OnElements("img")
	policy.AllowAttrs("src").Matching(dataImageSrcPattern).OnElements("img")
	policy.AllowAttrs("align").OnElements("div", "h1", "h2", "h3", "h4", "h5", "h6", "p", "table", "td", "th", "tr")
	policy.AllowAttrs("bgcolor", "border", "cellpadding", "cellspacing", "height", "valign", "width").OnElements("table", "td", "th", "tr")
	policy.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	policy.AllowAttrs("color", "face", "size").OnElements("font")
	policy.AllowDataURIImages()
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

func resolveCIDImageSources(value string, images map[string]inlineImage) string {
	if len(images) == 0 || value == "" {
		return value
	}
	return cidImageSrcPattern.ReplaceAllStringFunc(value, func(match string) string {
		parts := cidImageSrcPattern.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		cid := normalizeContentID(strings.Trim(parts[2], `"'`))
		image, ok := images[cid]
		if !ok || !strings.HasPrefix(strings.ToLower(image.mediaType), "image/") {
			return match
		}
		src := "data:" + image.mediaType + ";base64," + base64.StdEncoding.EncodeToString(image.data)
		return parts[1] + ` src="` + html.EscapeString(src) + `"`
	})
}

func isInlineCIDImage(header mail.Header, mediaType string, disposition string) bool {
	if header.Get("Content-ID") == "" || !strings.HasPrefix(strings.ToLower(mediaType), "image/") {
		return false
	}
	return disposition == "" || strings.EqualFold(disposition, "inline")
}

func normalizeContentID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 4 && strings.EqualFold(value[:4], "cid:") {
		value = value[4:]
	}
	value = strings.Trim(value, "<>")
	if unescaped, err := url.PathUnescape(value); err == nil {
		value = unescaped
	}
	return strings.ToLower(value)
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

func parseAddressListHeaders(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	parser := mail.AddressParser{WordDecoder: &mime.WordDecoder{}}
	addresses, err := parser.ParseList(strings.Join(values, ","))
	if err != nil {
		return values
	}
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		name := decodeHeader(address.Name)
		if name == "" {
			result = append(result, address.Address)
			continue
		}
		result = append(result, name+" <"+address.Address+">")
	}
	return result
}

func parseMessageIDList(value string) []string {
	matches := messageIDPattern.FindAllStringSubmatch(value, -1)
	if len(matches) > 0 {
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			if id := normalizeMessageID(match[1]); id != "" {
				ids = append(ids, id)
			}
		}
		return ids
	}
	fields := strings.Fields(value)
	ids := make([]string, 0, len(fields))
	for _, field := range fields {
		if id := normalizeMessageID(field); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func normalizeMessageID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "<>")
	value = strings.TrimSpace(value)
	return strings.ToLower(value)
}

func threadIDFromHeaders(messageID string, inReplyTo string, references []string) string {
	if len(references) > 0 {
		return references[0]
	}
	if inReplyTo != "" {
		return inReplyTo
	}
	return messageID
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
