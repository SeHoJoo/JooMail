package httpapi

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestHealth(t *testing.T) {
	recorder := request(t, http.MethodGet, "/api/health", nil, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body map[string]string
	decode(t, recorder, &body)
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestProtectedRoutesRejectMissingSession(t *testing.T) {
	for _, test := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/accounts", ""},
		{http.MethodGet, "/api/accounts/jooseho@good-night.co.kr/mailboxes/inbox/messages", ""},
		{http.MethodGet, "/api/messages/inbox.1", ""},
		{http.MethodGet, "/api/messages/inbox.1/attachments/att_0", ""},
		{http.MethodPatch, "/api/messages/inbox.1/flagged", `{"flagged":true}`},
		{http.MethodPatch, "/api/messages/inbox.1/seen", `{"seen":false}`},
		{http.MethodPost, "/api/messages/inbox.1/move", `{"mailboxId":"mbox_QXJjaGl2ZQ"}`},
		{http.MethodPost, "/api/send", `{"to":["a@example.com"],"subject":"Hi","textBody":"Hello"}`},
		{http.MethodPost, "/api/logout", ""},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			recorder := request(t, test.method, test.path, strings.NewReader(test.body), nil)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestProtectedRoutesRejectInvalidSession(t *testing.T) {
	config := testConfig(t, "127.0.0.1", "1")
	server := NewServerWithConfig(MockStore(), config)
	cookie := &http.Cookie{Name: "joomail_session", Value: "invalid.token"}

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginStoresEncryptedCredentialAndSetsRememberedSessionCookie(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string {
			if username != "jooseho" || password != "correct-password" {
				t.Fatalf("login credentials = %q/%q, want jooseho/correct-password", username, password)
			}
			return "OK LOGIN completed"
		},
	})
	config := testConfig(t, host, port)
	server := NewServerWithConfig(MockStore(), config)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"jooseho@good-night.co.kr","password":"correct-password","remember":true}`))

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Account Account `json:"account"`
	}
	decode(t, recorder, &body)
	if body.Account.Email != "jooseho@good-night.co.kr" {
		t.Fatalf("account email = %q, want submitted email", body.Account.Email)
	}

	cookie := sessionCookie(t, recorder)
	if cookie.MaxAge < int((29 * 24 * time.Hour).Seconds()) {
		t.Fatalf("cookie MaxAge = %d, want remembered session", cookie.MaxAge)
	}

	payload, err := verifySessionToken(cookie.Value, "test-session-secret")
	if err != nil {
		t.Fatalf("verify session token: %v", err)
	}
	if payload.SessionID == "" || !payload.Remember || payload.Email != "jooseho@good-night.co.kr" {
		t.Fatalf("session payload = %#v", payload)
	}

	files := credentialFiles(t, config.CredentialDir)
	if len(files) != 1 {
		t.Fatalf("credential file count = %d, want 1", len(files))
	}
	contents := readFile(t, files[0])
	if bytes.Contains(contents, []byte("correct-password")) {
		t.Fatal("credential file contains plaintext password")
	}

	store, err := newCredentialStore(config)
	if err != nil {
		t.Fatalf("new credential store: %v", err)
	}
	credential, err := store.Load(payload.SessionID, payload.Email)
	if err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if credential.Password != "correct-password" || credential.IMAPUsername != "jooseho" {
		t.Fatalf("credential = %#v", credential)
	}
}

func TestLoginRejectsWrongEmailDomainBeforeIMAPLogin(t *testing.T) {
	var loginCalled bool
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string {
			loginCalled = true
			return "OK LOGIN completed"
		},
	})
	config := testConfig(t, host, port)
	config.LoginDomain = "good-night.co.kr"
	server := NewServerWithConfig(MockStore(), config)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"jooseho@naver.com","password":"correct-password","remember":true}`))

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusUnauthorized, recorder.Body.String())
	}
	if loginCalled {
		t.Fatal("IMAP login was called for disallowed email domain")
	}
	if len(recorder.Result().Cookies()) != 0 {
		t.Fatalf("cookies = %#v, want none", recorder.Result().Cookies())
	}
}

func TestConfiguredLoginDomainDerivesFromMailHost(t *testing.T) {
	config := Config{IMAPHost: "mail.good-night.co.kr"}
	if domain := configuredLoginDomain(config); domain != "good-night.co.kr" {
		t.Fatalf("domain = %q, want good-night.co.kr", domain)
	}
}

func TestSessionUsesStoredCredentialForMailboxAndMessageRoutes(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: MIME parsed by backend",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello from IMAP.",
		"",
		"--abc",
		"Content-Type: application/pdf; name=\"roadmap.pdf\"",
		"Content-Disposition: attachment; filename=\"roadmap.pdf\"",
		"Content-Transfer-Encoding: base64",
		"",
		"cGRm",
		"--abc--",
		"",
	}, "\r\n")
	var loginCount int
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string {
			loginCount++
			if username != "jooseho" || password != "correct-password" {
				t.Fatalf("route login credentials = %q/%q", username, password)
			}
			return "OK LOGIN completed"
		},
		mailboxes: []string{"INBOX", "Sent"},
		messages:  map[string]map[string]string{"INBOX": {"7": rawMessage}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	accountsRecorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)
	if accountsRecorder.Code != http.StatusOK {
		t.Fatalf("accounts status = %d, want %d; body = %s", accountsRecorder.Code, http.StatusOK, accountsRecorder.Body.String())
	}
	var accountsBody struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, accountsRecorder, &accountsBody)
	if len(accountsBody.Accounts) != 1 || accountsBody.Accounts[0].Email != "jooseho@good-night.co.kr" {
		t.Fatalf("accounts = %#v", accountsBody.Accounts)
	}

	messagesRecorder := requestWithServer(t, server, http.MethodGet, "/api/accounts/jooseho%40good-night.co.kr/mailboxes/inbox/messages", nil, cookie)
	if messagesRecorder.Code != http.StatusOK {
		t.Fatalf("messages status = %d, want %d; body = %s", messagesRecorder.Code, http.StatusOK, messagesRecorder.Body.String())
	}
	var messagesBody struct {
		Messages []MessageSummary `json:"messages"`
	}
	decode(t, messagesRecorder, &messagesBody)
	if len(messagesBody.Messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(messagesBody.Messages))
	}
	if messagesBody.Messages[0].Subject != "MIME parsed by backend" || !messagesBody.Messages[0].HasAttachment {
		t.Fatalf("summary = %#v", messagesBody.Messages[0])
	}

	messageRecorder := requestWithServer(t, server, http.MethodGet, "/api/messages/"+messagesBody.Messages[0].ID, nil, cookie)
	if messageRecorder.Code != http.StatusOK {
		t.Fatalf("message status = %d, want %d; body = %s", messageRecorder.Code, http.StatusOK, messageRecorder.Body.String())
	}
	var messageBody struct {
		Message Message `json:"message"`
	}
	decode(t, messageRecorder, &messageBody)
	if strings.Join(messageBody.Message.TextBody, "\n") != "Hello from IMAP." {
		t.Fatalf("textBody = %#v", messageBody.Message.TextBody)
	}
	if len(messageBody.Message.Attachments) != 1 || messageBody.Message.Attachments[0].ID != "att_0" || messageBody.Message.Attachments[0].Name != "roadmap.pdf" {
		t.Fatalf("attachments = %#v", messageBody.Message.Attachments)
	}
	if loginCount < 3 {
		t.Fatalf("login count = %d, want each authenticated route to open IMAP", loginCount)
	}
}

func TestMessageAttachmentRouteDownloadsDecodedAttachment(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Attachment",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello.",
		"",
		"--abc",
		"Content-Type: text/plain; name=\"notes.txt\"",
		"Content-Disposition: attachment; filename=\"notes.txt\"",
		"Content-Transfer-Encoding: base64",
		"",
		"Tm90ZSBib2R5",
		"--abc--",
		"",
	}, "\r\n")
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin:  func(username, password string) string { return "OK LOGIN completed" },
		messages: map[string]map[string]string{"INBOX": {"7": rawMessage}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))
	messageID := messageID("inbox", "7")

	recorder := requestWithServer(t, server, http.MethodGet, "/api/messages/"+messageID+"/attachments/att_0", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Body.String() != "Note body" {
		t.Fatalf("attachment body = %q, want decoded body", recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "notes.txt") {
		t.Fatalf("content disposition = %q, want filename", recorder.Header().Get("Content-Disposition"))
	}
	if recorder.Header().Get("Content-Type") != "text/plain" {
		t.Fatalf("content type = %q, want text/plain", recorder.Header().Get("Content-Type"))
	}
}

func TestAccountsSkipNoselectNamespaceRootMailbox(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		mailboxLines: []string{
			`* LIST (\Noselect \HasChildren) "." "."`,
			`* LIST () "." "INBOX"`,
			`* LIST () "." "Sent"`,
		},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	if len(body.Accounts) != 1 {
		t.Fatalf("accounts = %#v", body.Accounts)
	}
	var mailboxIDs []string
	for _, mailbox := range body.Accounts[0].Mailboxes {
		mailboxIDs = append(mailboxIDs, mailbox.ID)
	}
	if strings.Contains(strings.Join(mailboxIDs, ","), "mbox_Lg") {
		t.Fatalf("mailbox IDs = %#v, should not include noselect namespace root", mailboxIDs)
	}
	if !containsString(mailboxIDs, "inbox") {
		t.Fatalf("mailbox IDs = %#v, want inbox", mailboxIDs)
	}
}

func TestAccountsParseUnquotedMailboxNamesWithDotDelimiter(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		mailboxLines: []string{
			`* LIST () "." INBOX`,
			`* LIST () "." Sent`,
			`* LIST () "." Archive.2026`,
		},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	if len(body.Accounts) != 1 {
		t.Fatalf("accounts = %#v", body.Accounts)
	}
	var labels []string
	for _, mailbox := range body.Accounts[0].Mailboxes {
		labels = append(labels, mailbox.Label)
		if mailbox.ID == "mbox_Lg" {
			t.Fatalf("mailbox = %#v, parsed delimiter as mailbox", mailbox)
		}
	}
	if !containsString(labels, "Archive.2026") {
		t.Fatalf("labels = %#v, want Archive.2026", labels)
	}
}

func TestAccountsBuildNestedMailboxTree(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		mailboxLines: []string{
			`* LIST () "/" "INBOX"`,
			`* LIST (\Noselect \HasChildren) "/" "Work"`,
			`* LIST () "/" "Work/Clients"`,
			`* LIST () "/" "Work/Internal"`,
			`* LIST () "/" "Work/Internal/Reports"`,
		},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	work, ok := findMailboxByLabel(body.Accounts[0].Mailboxes, "Work")
	if !ok {
		t.Fatalf("mailboxes = %#v, want Work parent", body.Accounts[0].Mailboxes)
	}
	if work.Selectable {
		t.Fatalf("Work selectable = true, want false for noselect parent")
	}
	if len(work.Children) != 2 {
		t.Fatalf("Work children = %#v, want Clients and Internal", work.Children)
	}
	internal, ok := findMailboxByLabel(work.Children, "Internal")
	if !ok {
		t.Fatalf("Work children = %#v, want Internal", work.Children)
	}
	reports, ok := findMailboxByLabel(internal.Children, "Reports")
	if !ok || !reports.Selectable {
		t.Fatalf("Internal children = %#v, want selectable Reports", internal.Children)
	}
}

func TestAccountsBuildNestedMailboxTreeWithDotDelimiter(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		mailboxLines: []string{
			`* LIST () "." INBOX`,
			`* LIST () "." Projects`,
			`* LIST () "." Projects.2026`,
		},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	projects, ok := findMailboxByLabel(body.Accounts[0].Mailboxes, "Projects")
	if !ok {
		t.Fatalf("mailboxes = %#v, want Projects parent", body.Accounts[0].Mailboxes)
	}
	child, ok := findMailboxByLabel(projects.Children, "2026")
	if !ok || !child.Selectable {
		t.Fatalf("Projects children = %#v, want selectable 2026", projects.Children)
	}
}

func TestAccountsDecodeModifiedUTF7MailboxNames(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		mailboxLines: []string{
			`* LIST () "/" "INBOX"`,
			`* LIST () "/" "&0UzCpNK4wMHHkA-"`,
		},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	if len(body.Accounts) != 1 {
		t.Fatalf("accounts = %#v", body.Accounts)
	}
	var labels []string
	for _, mailbox := range body.Accounts[0].Mailboxes {
		labels = append(labels, mailbox.Label)
	}
	if !containsString(labels, "테스트상자") {
		t.Fatalf("labels = %#v, want decoded Korean mailbox label", labels)
	}
}

func TestParseRawMessageSplitsAddressListHeaders(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>, Bob <bob@example.com>",
		"Cc: Carol <carol@example.com>, dave@example.com",
		"Subject: Reply all addresses",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello",
	}, "\r\n")

	message, err := parseRawMessage("jooseho@good-night.co.kr", "inbox", "7", []byte(raw))

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if got, want := message.Headers.To, []string{"Jooseho <jooseho@good-night.co.kr>", "Bob <bob@example.com>"}; !slicesEqual(got, want) {
		t.Fatalf("to = %#v, want %#v", got, want)
	}
	if got, want := message.Headers.Cc, []string{"Carol <carol@example.com>", "dave@example.com"}; !slicesEqual(got, want) {
		t.Fatalf("cc = %#v, want %#v", got, want)
	}
}

func TestAccountsOrderInboxFirst(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin:   func(username, password string) string { return "OK LOGIN completed" },
		mailboxes: []string{"Trash", "Sent", "INBOX", "Archive"},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	if len(body.Accounts) != 1 || len(body.Accounts[0].Mailboxes) == 0 {
		t.Fatalf("accounts = %#v", body.Accounts)
	}
	if body.Accounts[0].Mailboxes[0].ID != "inbox" {
		t.Fatalf("first mailbox = %#v, want inbox first", body.Accounts[0].Mailboxes[0])
	}
}

func TestMessageSummariesOrderNewestFirst(t *testing.T) {
	oldMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Old message",
		"Date: Thu, 2 Jul 2026 08:00:00 +0900",
		"",
		"Older body.",
	}, "\r\n")
	newMessage := strings.Join([]string{
		"From: Bob <bob@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: New message",
		"Date: Thu, 2 Jul 2026 09:00:00 +0900",
		"",
		"Newer body.",
	}, "\r\n")
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin:             func(username, password string) string { return "OK LOGIN completed" },
		fetchResponsesAsc:   true,
		messages:            map[string]map[string]string{"INBOX": {"1": oldMessage, "2": newMessage}},
		orderedSearchResult: []string{"1", "2"},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts/jooseho%40good-night.co.kr/mailboxes/inbox/messages", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Messages []MessageSummary `json:"messages"`
	}
	decode(t, recorder, &body)
	if len(body.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(body.Messages))
	}
	if body.Messages[0].Subject != "New message" {
		t.Fatalf("subjects = %#v, want newest first", []string{body.Messages[0].Subject, body.Messages[1].Subject})
	}
}

func TestMessageSummariesParseUnreadAndFlaggedFromIMAPFlags(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Flagged unread message",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"",
		"Hello from IMAP.",
	}, "\r\n")
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin:  func(username, password string) string { return "OK LOGIN completed" },
		messages: map[string]map[string]string{"INBOX": {"7": rawMessage}},
		flags:    map[string]map[string]string{"INBOX": {"7": `(\Flagged)`}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts/jooseho%40good-night.co.kr/mailboxes/inbox/messages", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var body struct {
		Messages []MessageSummary `json:"messages"`
	}
	decode(t, recorder, &body)
	if len(body.Messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(body.Messages))
	}
	if !body.Messages[0].Unread || !body.Messages[0].Flagged {
		t.Fatalf("message flags = unread %v flagged %v, want true/true", body.Messages[0].Unread, body.Messages[0].Flagged)
	}
}

func TestMessageSummariesFetchFlagsFromIMAP(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Read message",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"",
		"Hello from IMAP.",
	}, "\r\n")
	var fetchDataItems string
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onFetch: func(mailbox string, uidSet string, dataItems string) {
			fetchDataItems = dataItems
		},
		messages: map[string]map[string]string{"INBOX": {"7": rawMessage}},
		flags:    map[string]map[string]string{"INBOX": {"7": `(\Seen)`}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts/jooseho%40good-night.co.kr/mailboxes/inbox/messages", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(strings.ToUpper(fetchDataItems), "FLAGS") {
		t.Fatalf("FETCH data items = %q, want FLAGS requested", fetchDataItems)
	}
	var body struct {
		Messages []MessageSummary `json:"messages"`
	}
	decode(t, recorder, &body)
	if len(body.Messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(body.Messages))
	}
	if body.Messages[0].Unread {
		t.Fatalf("unread = true, want false from server \\Seen flag")
	}
}

func TestMessageDetailMarksUnreadMessageSeen(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Unread message",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"",
		"Hello from IMAP.",
	}, "\r\n")
	var storedUID string
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onStore: func(mailbox string, uid string, operation string, flag string) string {
			storedUID = uid
			return "OK STORE completed"
		},
		messages: map[string]map[string]string{"INBOX": {"7": rawMessage}},
		flags:    map[string]map[string]string{"INBOX": {"7": `()`}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))
	summaryRecorder := requestWithServer(t, server, http.MethodGet, "/api/accounts/jooseho%40good-night.co.kr/mailboxes/inbox/messages", nil, cookie)
	if summaryRecorder.Code != http.StatusOK {
		t.Fatalf("summary status = %d, want %d; body = %s", summaryRecorder.Code, http.StatusOK, summaryRecorder.Body.String())
	}
	var summaryBody struct {
		Messages []MessageSummary `json:"messages"`
	}
	decode(t, summaryRecorder, &summaryBody)
	if len(summaryBody.Messages) != 1 || !summaryBody.Messages[0].Unread {
		t.Fatalf("summaries = %#v, want unread message", summaryBody.Messages)
	}

	detailRecorder := requestWithServer(t, server, http.MethodGet, "/api/messages/"+summaryBody.Messages[0].ID, nil, cookie)

	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d; body = %s", detailRecorder.Code, http.StatusOK, detailRecorder.Body.String())
	}
	if storedUID != "7" {
		t.Fatalf("stored UID = %q, want 7", storedUID)
	}
	var detailBody struct {
		Message Message `json:"message"`
	}
	decode(t, detailRecorder, &detailBody)
	if detailBody.Message.Unread {
		t.Fatalf("detail unread = true, want false after marking seen")
	}
}

func TestMessageFlaggedRouteStoresFlaggedFlag(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Flag me",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"",
		"Hello from IMAP.",
	}, "\r\n")
	var storedMailbox string
	var storedUID string
	var storedOperation string
	var storedFlag string
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onStore: func(mailbox string, uid string, operation string, flag string) string {
			storedMailbox = mailbox
			storedUID = uid
			storedOperation = operation
			storedFlag = flag
			return "OK STORE completed"
		},
		messages: map[string]map[string]string{"INBOX": {"7": rawMessage}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))
	messageID := messageID("inbox", "7")

	recorder := requestWithServer(t, server, http.MethodPatch, "/api/messages/"+messageID+"/flagged", strings.NewReader(`{"flagged":true}`), cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if storedMailbox != "INBOX" || storedUID != "7" || storedOperation != "+FLAGS.SILENT" || storedFlag != `(\Flagged)` {
		t.Fatalf("store = mailbox %q uid %q operation %q flag %q", storedMailbox, storedUID, storedOperation, storedFlag)
	}
	var body struct {
		Flagged bool `json:"flagged"`
	}
	decode(t, recorder, &body)
	if !body.Flagged {
		t.Fatal("flagged = false, want true")
	}
}

func TestMessageFlaggedRouteClearsFlaggedFlag(t *testing.T) {
	var storedOperation string
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onStore: func(mailbox string, uid string, operation string, flag string) string {
			storedOperation = operation
			return "OK STORE completed"
		},
		messages: map[string]map[string]string{"INBOX": {"7": "From: Alice <alice@example.com>\r\nSubject: Flagged\r\n\r\nBody"}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))
	messageID := messageID("inbox", "7")

	recorder := requestWithServer(t, server, http.MethodPatch, "/api/messages/"+messageID+"/flagged", strings.NewReader(`{"flagged":false}`), cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if storedOperation != "-FLAGS.SILENT" {
		t.Fatalf("operation = %q, want -FLAGS.SILENT", storedOperation)
	}
}

func TestMessageSeenRouteClearsSeenFlag(t *testing.T) {
	var storedOperation string
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onStore: func(mailbox string, uid string, operation string, flag string) string {
			storedOperation = operation
			return "OK STORE completed"
		},
		messages: map[string]map[string]string{"INBOX": {"7": "From: Alice <alice@example.com>\r\nSubject: Seen\r\n\r\nBody"}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))
	messageID := messageID("inbox", "7")

	recorder := requestWithServer(t, server, http.MethodPatch, "/api/messages/"+messageID+"/seen", strings.NewReader(`{"seen":false}`), cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if storedOperation != "-FLAGS.SILENT" {
		t.Fatalf("operation = %q, want -FLAGS.SILENT", storedOperation)
	}
}

func TestMessageMoveRouteMovesMessageToTargetMailbox(t *testing.T) {
	var movedUID string
	var movedTarget string
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onMove: func(mailbox string, uid string, target string) string {
			movedUID = uid
			movedTarget = target
			return "OK MOVE completed"
		},
		messages: map[string]map[string]string{"INBOX": {"7": "From: Alice <alice@example.com>\r\nSubject: Move\r\n\r\nBody"}},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))
	messageID := messageID("inbox", "7")

	recorder := requestWithServer(t, server, http.MethodPost, "/api/messages/"+messageID+"/move", strings.NewReader(`{"mailboxId":"mbox_QXJjaGl2ZQ"}`), cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if movedUID != "7" || movedTarget != "Archive" {
		t.Fatalf("move = uid %q target %q, want 7/Archive", movedUID, movedTarget)
	}
}

func TestParseRawMessageFixture(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: MIME parsed by backend",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/mixed; boundary=abc",
		"",
		"--abc",
		"Content-Type: text/plain; charset=utf-8",
		"",
		"Hello from IMAP.",
		"",
		"--abc",
		"Content-Type: application/pdf; name=\"roadmap.pdf\"",
		"Content-Disposition: attachment; filename=\"roadmap.pdf\"",
		"Content-Transfer-Encoding: base64",
		"",
		"cGRm",
		"--abc--",
		"",
	}, "\r\n")
	message, err := parseRawMessage("account", "inbox", "7", []byte(rawMessage))
	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if message.Subject != "MIME parsed by backend" {
		t.Fatalf("subject = %q", message.Subject)
	}
}

func TestParseRawMessageDecodesNonUTF8Charset(t *testing.T) {
	raw := bytes.NewBuffer(nil)
	raw.WriteString(strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: =?EUC-KR?B?vsiz58fPvLy/5A==?=",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: text/plain; charset=EUC-KR",
		"",
	}, "\r\n"))
	raw.WriteString("\r\n")
	raw.Write([]byte{0xbe, 0xc8, 0xb3, 0xe7, 0xc7, 0xcf, 0xbc, 0xbc, 0xbf, 0xe4})

	message, err := parseRawMessage("account", "inbox", "8", raw.Bytes())

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if message.Subject != "안녕하세요" {
		t.Fatalf("subject = %q, want decoded EUC-KR", message.Subject)
	}
	if strings.Join(message.TextBody, "\n") != "안녕하세요" {
		t.Fatalf("textBody = %#v, want decoded EUC-KR", message.TextBody)
	}
}

func TestParseRawMessageMultipartAlternativeSanitizesHTML(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Alternative body",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/alternative; boundary=alt",
		"",
		"--alt",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Plain=20fallback.",
		"",
		"--alt",
		"Content-Type: text/html; charset=utf-8",
		"Content-Transfer-Encoding: base64",
		"",
		"PHA+SGVsbG8gPHN0cm9uZz5zYWZlPC9zdHJvbmc+PC9wPjxzY3JpcHQ+YWxlcnQoMSk8L3NjcmlwdD48aW1nIHNyYz0iaHR0cHM6Ly90cmFja2VyLmV4YW1wbGUvcGl4ZWwucG5nIiBvbmVycm9yPSJhbGVydCgxKSI+",
		"--alt--",
		"",
	}, "\r\n")

	message, err := parseRawMessage("account", "inbox", "8", []byte(rawMessage))

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if strings.Join(message.TextBody, "\n") != "Plain fallback." {
		t.Fatalf("textBody = %#v", message.TextBody)
	}
	if !strings.Contains(message.HTMLBody, "Hello") || !strings.Contains(message.HTMLBody, "safe") {
		t.Fatalf("htmlBody = %q, want sanitized HTML body", message.HTMLBody)
	}
	for _, unsafe := range []string{"script", "alert", "onerror"} {
		if strings.Contains(strings.ToLower(message.HTMLBody), unsafe) {
			t.Fatalf("htmlBody = %q, contains unsafe content %q", message.HTMLBody, unsafe)
		}
	}
	if strings.Contains(message.HTMLBody, "<img src=\"https://tracker.example/pixel.png\"") {
		t.Fatalf("htmlBody = %q, remote image src should be blocked until requested", message.HTMLBody)
	}
	if !strings.Contains(message.HTMLBody, "data-joomail-remote-src=\"https://tracker.example/pixel.png\"") {
		t.Fatalf("htmlBody = %q, want remote image URL preserved in data attribute", message.HTMLBody)
	}
	if !message.RemoteImagesBlocked {
		t.Fatal("remoteImagesBlocked = false, want true for remote image HTML")
	}
}

func TestParseRawMessageMultipartAlternativeFallsBackToTextOnly(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Text alternative",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/alternative; boundary=alt",
		"",
		"--alt",
		"Content-Type: text/plain; charset=utf-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		"Plain=20fallback.",
		"--alt--",
		"",
	}, "\r\n")

	message, err := parseRawMessage("account", "inbox", "8", []byte(rawMessage))

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if strings.Join(message.TextBody, "\n") != "Plain fallback." {
		t.Fatalf("textBody = %#v, want plain fallback", message.TextBody)
	}
	if message.HTMLBody != "" {
		t.Fatalf("htmlBody = %q, want no HTML fallback", message.HTMLBody)
	}
}

func TestParseRawMessageMultipartRelatedMapsCIDImages(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Inline image",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/related; boundary=rel",
		"",
		"--rel",
		"Content-Type: text/html; charset=utf-8",
		"",
		`<p>Logo <img src="cid:logo.123@example" onload="alert(1)"></p>`,
		"",
		"--rel",
		"Content-Type: image/png; name=\"logo.png\"",
		"Content-ID: <logo.123@example>",
		"Content-Disposition: inline; filename=\"logo.png\"",
		"Content-Transfer-Encoding: base64",
		"",
		"iVBORw0KGgo=",
		"--rel--",
		"",
	}, "\r\n")

	message, err := parseRawMessage("account", "inbox", "8", []byte(rawMessage))

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if !strings.Contains(message.HTMLBody, `src="data:image/png;base64,iVBORw0KGgo="`) {
		t.Fatalf("htmlBody = %q, want cid image mapped to data URL", message.HTMLBody)
	}
	if strings.Contains(strings.ToLower(message.HTMLBody), "onload") || strings.Contains(message.HTMLBody, "cid:") {
		t.Fatalf("htmlBody = %q, want sanitized resolved image", message.HTMLBody)
	}
	if message.RemoteImagesBlocked {
		t.Fatal("remoteImagesBlocked = true, want cid image not treated as remote image")
	}
	if len(message.Attachments) != 0 {
		t.Fatalf("attachments = %#v, want inline cid image excluded from attachment list", message.Attachments)
	}
}

func TestParseRawMessageTransferEncodings(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		body     string
		want     string
	}{
		{name: "base64", encoding: "base64", body: "QmFzZTY0IGJvZHku", want: "Base64 body."},
		{name: "quoted-printable", encoding: "quoted-printable", body: "Quoted=20printable=20body.", want: "Quoted printable body."},
		{name: "7bit", encoding: "7bit", body: "Seven bit body.", want: "Seven bit body."},
		{name: "8bit", encoding: "8bit", body: "Cafe \xc3\xa9 body.", want: "Cafe é body."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawMessage := strings.Join([]string{
				"From: Alice <alice@example.com>",
				"To: Jooseho <jooseho@good-night.co.kr>",
				"Subject: Transfer",
				"Date: Thu, 2 Jul 2026 09:14:00 +0900",
				"Content-Type: text/plain; charset=utf-8",
				"Content-Transfer-Encoding: " + tt.encoding,
				"",
				tt.body,
			}, "\r\n")

			message, err := parseRawMessage("account", "inbox", "8", []byte(rawMessage))

			if err != nil {
				t.Fatalf("parse raw message: %v", err)
			}
			if strings.Join(message.TextBody, "\n") != tt.want {
				t.Fatalf("textBody = %#v, want %q", message.TextBody, tt.want)
			}
		})
	}
}

func TestParseRawMessageDecodesISO2022JPCharset(t *testing.T) {
	body, err := io.ReadAll(transform.NewReader(strings.NewReader("日本語本文"), japanese.ISO2022JP.NewEncoder()))
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	raw := bytes.NewBuffer(nil)
	raw.WriteString(strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: =?ISO-2022-JP?B?GyRCRnxLXDhsGyhC?=",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: text/plain; charset=ISO-2022-JP",
		"",
	}, "\r\n"))
	raw.WriteString("\r\n")
	raw.Write(body)

	message, err := parseRawMessage("account", "inbox", "8", raw.Bytes())

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if message.Subject != "日本語" {
		t.Fatalf("subject = %q, want decoded ISO-2022-JP", message.Subject)
	}
	if strings.Join(message.TextBody, "\n") != "日本語本文" {
		t.Fatalf("textBody = %#v, want decoded ISO-2022-JP", message.TextBody)
	}
}

func TestParseRawMessageUnsupportedCharsetKeepsVisibleFallback(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Unsupported charset",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: text/plain; charset=x-unknown-mailer",
		"",
		"Visible fallback body.",
	}, "\r\n")

	message, err := parseRawMessage("account", "inbox", "8", []byte(rawMessage))

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if strings.Join(message.TextBody, "\n") != "Visible fallback body." {
		t.Fatalf("textBody = %#v, want raw visible fallback", message.TextBody)
	}
}

func TestParseRawMessageMalformedMultipartKeepsUsableHeaders(t *testing.T) {
	rawMessage := strings.Join([]string{
		"From: Broken Sender <broken@example.com>",
		"To: Jooseho <jooseho@good-night.co.kr>",
		"Subject: Broken multipart",
		"Date: Thu, 2 Jul 2026 09:14:00 +0900",
		"Content-Type: multipart/mixed",
		"",
		"This body cannot be parsed as multipart.",
	}, "\r\n")

	message, err := parseRawMessage("account", "inbox", "8", []byte(rawMessage))

	if err != nil {
		t.Fatalf("parse raw message: %v", err)
	}
	if message.Subject != "Broken multipart" || message.SenderEmail != "broken@example.com" || message.Headers.Date == "" {
		t.Fatalf("message headers = %#v", message)
	}
}

func TestIMAPProtocolErrorsReturnGenericResponse(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
		onSelect: func(mailbox string) string {
			return "NO secret backend failure text"
		},
	})
	server, cookie := loginTestSession(t, testConfig(t, host, port))

	recorder := requestWithServer(t, server, http.MethodGet, "/api/accounts/jooseho%40good-night.co.kr/mailboxes/inbox/messages", nil, cookie)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusBadGateway, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret backend failure text") {
		t.Fatalf("response leaked upstream error: %s", recorder.Body.String())
	}
}

func TestSendUsesStoredCredentialForSMTP(t *testing.T) {
	var appendedMailbox string
	var appendedMessage string
	imapHost, imapPort := startFakeIMAPServer(t, fakeIMAPScript{
		mailboxes: []string{"INBOX", "Sent"},
		onLogin: func(username, password string) string {
			if username != "jooseho" || password != "mail-password" {
				t.Fatalf("imap login credentials = %q/%q", username, password)
			}
			return "OK LOGIN completed"
		},
		onAppend: func(mailbox string, message string) string {
			appendedMailbox = mailbox
			appendedMessage = message
			return "OK APPEND completed"
		},
	})
	var smtpAuthLine string
	var smtpData string
	smtpHost, smtpPort := startFakeSMTPServer(t, &smtpAuthLine, &smtpData)
	config := testConfig(t, imapHost, imapPort)
	config.SMTPHost = smtpHost
	config.SMTPPort = smtpPort
	config.SMTPUserFormat = "localpart"
	server, cookie := loginTestSessionWithPassword(t, config, "mail-password")

	recorder := requestWithServer(t, server, http.MethodPost, "/api/send", strings.NewReader(`{"to":["alice@example.com"],"subject":"Hello","textBody":"Plain message"}`), cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	authPayload := decodeSMTPPlainAuth(t, smtpAuthLine)
	if authPayload != "\x00jooseho\x00mail-password" {
		t.Fatalf("smtp auth payload = %q", authPayload)
	}
	if !strings.Contains(smtpData, "Subject: Hello") || !strings.Contains(smtpData, "Plain message") {
		t.Fatalf("smtp data = %q", smtpData)
	}
	if appendedMailbox != "Sent" {
		t.Fatalf("appended mailbox = %q, want Sent", appendedMailbox)
	}
	if normalizeMailLineEndings(appendedMessage) != normalizeMailLineEndings(smtpData) {
		t.Fatalf("appended message = %q, want SMTP data %q", appendedMessage, smtpData)
	}
}

func TestSendAcceptsMultipartAttachments(t *testing.T) {
	var appendedMessage string
	imapHost, imapPort := startFakeIMAPServer(t, fakeIMAPScript{
		mailboxes: []string{"INBOX", "Sent"},
		onLogin:   func(username, password string) string { return "OK LOGIN completed" },
		onAppend: func(mailbox string, message string) string {
			appendedMessage = message
			return "OK APPEND completed"
		},
	})
	var smtpAuthLine string
	var smtpData string
	smtpHost, smtpPort := startFakeSMTPServer(t, &smtpAuthLine, &smtpData)
	config := testConfig(t, imapHost, imapPort)
	config.SMTPHost = smtpHost
	config.SMTPPort = smtpPort
	config.SMTPUserFormat = "localpart"
	server, cookie := loginTestSessionWithPassword(t, config, "mail-password")
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("to", `["alice@example.com"]`)
	_ = writer.WriteField("subject", "With attachment")
	_ = writer.WriteField("textBody", "See attached.")
	part, err := writer.CreateFormFile("attachments", "notes.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("Hello attachment"))
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/send", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	for _, want := range []string{
		"Content-Type: multipart/mixed",
		"Content-Disposition: attachment; filename=notes.txt",
		"Content-Transfer-Encoding: base64",
		"SGVsbG8gYXR0YWNobWVudA==",
	} {
		if !strings.Contains(smtpData, want) {
			t.Fatalf("smtp data missing %q: %s", want, smtpData)
		}
	}
	if normalizeMailLineEndings(appendedMessage) != normalizeMailLineEndings(smtpData) {
		t.Fatalf("appended message = %q, want SMTP data %q", appendedMessage, smtpData)
	}
}

func TestSendUsesImplicitTLSForSMTPSPort(t *testing.T) {
	imapHost, imapPort := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
	})
	var smtpAuthLine string
	var smtpData string
	smtpHost, smtpPort := startFakeImplicitTLSSMTPServer(t, &smtpAuthLine, &smtpData)
	config := testConfig(t, imapHost, imapPort)
	config.SMTPHost = smtpHost
	config.SMTPPort = smtpPort
	config.SMTPTLS = true
	config.SMTPStartTLS = true
	config.SMTPUserFormat = "localpart"
	server, cookie := loginTestSessionWithPassword(t, config, "mail-password")

	originalSMTPTLSConfig := newSMTPTLSConfig
	newSMTPTLSConfig = func(host string) *tls.Config {
		return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	}
	t.Cleanup(func() { newSMTPTLSConfig = originalSMTPTLSConfig })

	recorder := requestWithServer(t, server, http.MethodPost, "/api/send", strings.NewReader(`{"to":["alice@example.com"],"subject":"Hello","textBody":"Plain message"}`), cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	authPayload := decodeSMTPPlainAuth(t, smtpAuthLine)
	if authPayload != "\x00jooseho\x00mail-password" {
		t.Fatalf("smtp auth payload = %q", authPayload)
	}
	if !strings.Contains(smtpData, "Plain message") {
		t.Fatalf("smtp data = %q", smtpData)
	}
}

func TestSMTPPort465UsesImplicitTLS(t *testing.T) {
	config := Config{SMTPPort: "465", SMTPStartTLS: true}

	if !smtpImplicitTLS(config) {
		t.Fatal("smtpImplicitTLS = false, want true for port 465")
	}
}

func TestFormatOutgoingMessageUsesConfiguredFromName(t *testing.T) {
	message := formatOutgoingMessage("jooseho@good-night.co.kr", sendRequest{
		FromName: "Jooseho Joo\r\nInjected: no",
		To:       []string{"alice@example.com"},
		Subject:  "Hello",
		TextBody: "Plain message",
	})

	if !strings.Contains(message, "From: Jooseho Joo  Injected: no <jooseho@good-night.co.kr>\r\n") {
		t.Fatalf("message = %q, want sanitized configured from name", message)
	}
}

func TestLogoutDeletesCredentialAndExpiresCookie(t *testing.T) {
	host, port := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
	})
	config := testConfig(t, host, port)
	server, cookie := loginTestSession(t, config)
	if len(credentialFiles(t, config.CredentialDir)) != 1 {
		t.Fatal("credential was not created")
	}

	recorder := requestWithServer(t, server, http.MethodPost, "/api/logout", nil, cookie)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(credentialFiles(t, config.CredentialDir)) != 0 {
		t.Fatal("credential was not deleted")
	}
	expired := sessionCookie(t, recorder)
	if expired.MaxAge >= 0 {
		t.Fatalf("logout cookie MaxAge = %d, want expired", expired.MaxAge)
	}
}

func TestStaticFilesKeepAPIRoutes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "app shell")

	server := WithStaticFiles(NewServer(MockStore()), dir)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestStaticFilesFallbackToIndex(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "index.html"), "app shell")

	server := WithStaticFiles(NewServer(MockStore()), dir)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/mail/personal/inbox", nil)
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.String() != "app shell" {
		t.Fatalf("body = %q, want app shell", recorder.Body.String())
	}
}

func request(t *testing.T, method string, path string, body io.Reader, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	return requestWithServer(t, NewServer(MockStore()), method, path, body, cookie)
}

func requestWithServer(t *testing.T, server http.Handler, method string, path string, body io.Reader, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, body)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	server.ServeHTTP(recorder, req)
	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(recorder.Body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return contents
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func slicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func findMailboxByLabel(mailboxes []Mailbox, label string) (Mailbox, bool) {
	for _, mailbox := range mailboxes {
		if mailbox.Label == label {
			return mailbox, true
		}
	}
	return Mailbox{}, false
}

func testConfig(t *testing.T, host string, port string) Config {
	t.Helper()
	key := bytes.Repeat([]byte{7}, 32)
	return Config{
		IMAPHost:       host,
		IMAPPort:       port,
		IMAPTLS:        false,
		IMAPUserFormat: "localpart",
		SMTPUserFormat: "localpart",
		SessionSecret:  "test-session-secret",
		CredentialKey:  base64.StdEncoding.EncodeToString(key),
		CredentialDir:  t.TempDir(),
	}
}

func loginTestSession(t *testing.T, config Config) (http.Handler, *http.Cookie) {
	t.Helper()
	return loginTestSessionWithPassword(t, config, "correct-password")
}

func loginTestSessionWithPassword(t *testing.T, config Config, password string) (http.Handler, *http.Cookie) {
	t.Helper()
	server := NewServerWithConfig(MockStore(), config)
	recorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"email":"jooseho@good-night.co.kr","password":%q,"remember":false}`, password)
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(body))
	server.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	return server, sessionCookie(t, recorder)
}

func sessionCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "joomail_session" {
			if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
				t.Fatalf("cookie security attributes = %#v", cookie)
			}
			return cookie
		}
	}
	t.Fatal("missing joomail_session cookie")
	return nil
}

func credentialFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read credential dir: %v", err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

type fakeIMAPScript struct {
	onLogin             func(username, password string) string
	onSelect            func(mailbox string) string
	onAppend            func(mailbox string, message string) string
	onStore             func(mailbox string, uid string, operation string, flag string) string
	onMove              func(mailbox string, uid string, target string) string
	onFetch             func(mailbox string, uidSet string, dataItems string)
	mailboxes           []string
	mailboxLines        []string
	messages            map[string]map[string]string
	flags               map[string]map[string]string
	orderedSearchResult []string
	fetchResponsesAsc   bool
}

func startFakeIMAPServer(t *testing.T, script fakeIMAPScript) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake imap: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleFakeIMAPConn(conn, script)
		}
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	return host, port
}

func handleFakeIMAPConn(conn net.Conn, script fakeIMAPScript) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	_, _ = conn.Write([]byte("* OK fake IMAP ready\r\n"))
	selectedMailbox := "INBOX"
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tag := fields[0]
		command := strings.ToUpper(fields[1])
		switch command {
		case "LOGIN":
			username := unquoteIMAPTestString(fields[2])
			password := unquoteIMAPTestString(fields[3])
			response := "OK LOGIN completed"
			if script.onLogin != nil {
				response = script.onLogin(username, password)
			}
			_, _ = conn.Write([]byte(tag + " " + response + "\r\n"))
		case "LIST":
			if len(script.mailboxLines) > 0 {
				for _, line := range script.mailboxLines {
					_, _ = conn.Write([]byte(line + "\r\n"))
				}
			} else {
				mailboxes := script.mailboxes
				if len(mailboxes) == 0 {
					mailboxes = []string{"INBOX"}
				}
				for _, mailbox := range mailboxes {
					_, _ = fmt.Fprintf(conn, "* LIST () \"/\" %q\r\n", mailbox)
				}
			}
			_, _ = conn.Write([]byte(tag + " OK LIST completed\r\n"))
		case "SELECT":
			if len(fields) >= 3 {
				selectedMailbox = unquoteIMAPTestString(fields[2])
			}
			if script.onSelect != nil {
				_, _ = conn.Write([]byte(tag + " " + script.onSelect(selectedMailbox) + "\r\n"))
				continue
			}
			count := len(script.messages[selectedMailbox])
			_, _ = fmt.Fprintf(conn, "* %d EXISTS\r\n%s OK SELECT completed\r\n", count, tag)
		case "UID":
			if len(fields) < 3 {
				_, _ = conn.Write([]byte(tag + " BAD UID command failed\r\n"))
				continue
			}
			switch strings.ToUpper(fields[2]) {
			case "SEARCH":
				uids := append([]string{}, script.orderedSearchResult...)
				if len(uids) == 0 {
					for uid := range script.messages[selectedMailbox] {
						uids = append(uids, uid)
					}
				}
				_, _ = fmt.Fprintf(conn, "* SEARCH %s\r\n%s OK SEARCH completed\r\n", strings.Join(uids, " "), tag)
			case "FETCH":
				if script.onFetch != nil {
					script.onFetch(selectedMailbox, fields[3], strings.Join(fields[4:], " "))
				}
				uidSet := strings.Split(fields[3], ",")
				if script.fetchResponsesAsc {
					sort.Strings(uidSet)
				}
				for _, uid := range uidSet {
					raw := script.messages[selectedMailbox][uid]
					if raw == "" {
						continue
					}
					flags := script.flags[selectedMailbox][uid]
					if flags == "" {
						flags = `()`
					}
					_, _ = fmt.Fprintf(conn, "* 1 FETCH (UID %s FLAGS %s BODY[] {%d}\r\n%s)\r\n", uid, flags, len(raw), raw)
				}
				_, _ = conn.Write([]byte(tag + " OK FETCH completed\r\n"))
			case "STORE":
				if len(fields) < 6 {
					_, _ = conn.Write([]byte(tag + " BAD STORE command failed\r\n"))
					continue
				}
				response := "OK STORE completed"
				if script.onStore != nil {
					response = script.onStore(selectedMailbox, fields[3], fields[4], fields[5])
				}
				_, _ = conn.Write([]byte(tag + " " + response + "\r\n"))
			case "MOVE":
				if len(fields) < 5 {
					_, _ = conn.Write([]byte(tag + " BAD MOVE command failed\r\n"))
					continue
				}
				response := "OK MOVE completed"
				if script.onMove != nil {
					response = script.onMove(selectedMailbox, fields[3], unquoteIMAPTestString(fields[4]))
				}
				_, _ = conn.Write([]byte(tag + " " + response + "\r\n"))
			case "COPY":
				_, _ = conn.Write([]byte(tag + " OK COPY completed\r\n"))
			}
		case "EXPUNGE":
			_, _ = conn.Write([]byte("* 1 EXPUNGE\r\n" + tag + " OK EXPUNGE completed\r\n"))
		case "APPEND":
			if len(fields) < 4 {
				_, _ = conn.Write([]byte(tag + " BAD APPEND command failed\r\n"))
				continue
			}
			mailbox := unquoteIMAPTestString(fields[2])
			size, ok := literalSize(line)
			if !ok {
				_, _ = conn.Write([]byte(tag + " BAD APPEND missing literal\r\n"))
				continue
			}
			_, _ = conn.Write([]byte("+ Ready for literal\r\n"))
			literal := make([]byte, size)
			if _, err := io.ReadFull(reader, literal); err != nil {
				return
			}
			_, _ = reader.ReadString('\n')
			response := "OK APPEND completed"
			if script.onAppend != nil {
				response = script.onAppend(mailbox, string(literal))
			}
			_, _ = conn.Write([]byte(tag + " " + response + "\r\n"))
		case "LOGOUT":
			_, _ = conn.Write([]byte("* BYE logging out\r\n" + tag + " OK LOGOUT completed\r\n"))
			return
		default:
			_, _ = conn.Write([]byte(tag + " OK completed\r\n"))
		}
	}
}

func unquoteIMAPTestString(value string) string {
	return strings.Trim(value, `"`)
}

func startFakeSMTPServer(t *testing.T, authLine *string, data *string) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake smtp: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("fake smtp server did not finish")
		}
	})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		serveFakeSMTPConn(conn, authLine, data)
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split smtp listener address: %v", err)
	}
	return host, port
}

func startFakeImplicitTLSSMTPServer(t *testing.T, authLine *string, data *string) (string, string) {
	t.Helper()
	certificate := newTestCertificate(t)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatalf("listen fake smtps: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("fake smtps server did not finish")
		}
	})

	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		serveFakeSMTPConn(conn, authLine, data)
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split smtps listener address: %v", err)
	}
	return host, port
}

func serveFakeSMTPConn(conn net.Conn, authLine *string, data *string) {
	reader := bufio.NewReader(conn)
	_, _ = conn.Write([]byte("220 fake smtp\r\n"))
	inData := false
	var dataLines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if inData {
			if line == "." {
				*data = strings.Join(dataLines, "\n")
				_, _ = conn.Write([]byte("250 queued\r\n"))
				inData = false
				continue
			}
			dataLines = append(dataLines, line)
			continue
		}
		switch {
		case strings.HasPrefix(line, "EHLO"):
			_, _ = conn.Write([]byte("250-localhost\r\n250 AUTH PLAIN\r\n"))
		case strings.HasPrefix(line, "AUTH PLAIN "):
			*authLine = line
			_, _ = conn.Write([]byte("235 authenticated\r\n"))
		case strings.HasPrefix(line, "MAIL FROM:"):
			_, _ = conn.Write([]byte("250 ok\r\n"))
		case strings.HasPrefix(line, "RCPT TO:"):
			_, _ = conn.Write([]byte("250 ok\r\n"))
		case line == "DATA":
			_, _ = conn.Write([]byte("354 send data\r\n"))
			inData = true
		case line == "QUIT":
			_, _ = conn.Write([]byte("221 bye\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("250 ok\r\n"))
		}
	}
}

func newTestCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func decodeSMTPPlainAuth(t *testing.T, line string) string {
	t.Helper()
	const prefix = "AUTH PLAIN "
	if !strings.HasPrefix(line, prefix) {
		t.Fatalf("smtp auth line = %q", line)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(line, prefix))
	if err != nil {
		t.Fatalf("decode smtp auth: %v", err)
	}
	return string(decoded)
}

func normalizeMailLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimSpace(value)
}
