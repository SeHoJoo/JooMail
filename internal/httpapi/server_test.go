package httpapi

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
	if len(messageBody.Message.Attachments) != 1 || messageBody.Message.Attachments[0].Name != "roadmap.pdf" {
		t.Fatalf("attachments = %#v", messageBody.Message.Attachments)
	}
	if loginCount < 3 {
		t.Fatalf("login count = %d, want each authenticated route to open IMAP", loginCount)
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

func TestSendUsesStoredCredentialForSMTP(t *testing.T) {
	imapHost, imapPort := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string {
			if username != "jooseho" || password != "mail-password" {
				t.Fatalf("imap login credentials = %q/%q", username, password)
			}
			return "OK LOGIN completed"
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
	onLogin   func(username, password string) string
	mailboxes []string
	messages  map[string]map[string]string
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
			mailboxes := script.mailboxes
			if len(mailboxes) == 0 {
				mailboxes = []string{"INBOX"}
			}
			for _, mailbox := range mailboxes {
				_, _ = fmt.Fprintf(conn, "* LIST () \"/\" %q\r\n", mailbox)
			}
			_, _ = conn.Write([]byte(tag + " OK LIST completed\r\n"))
		case "SELECT":
			if len(fields) >= 3 {
				selectedMailbox = unquoteIMAPTestString(fields[2])
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
				var uids []string
				for uid := range script.messages[selectedMailbox] {
					uids = append(uids, uid)
				}
				_, _ = fmt.Fprintf(conn, "* SEARCH %s\r\n%s OK SEARCH completed\r\n", strings.Join(uids, " "), tag)
			case "FETCH":
				uidSet := strings.Split(fields[3], ",")
				for _, uid := range uidSet {
					raw := script.messages[selectedMailbox][uid]
					if raw == "" {
						continue
					}
					_, _ = fmt.Fprintf(conn, "* 1 FETCH (UID %s BODY[] {%d}\r\n%s)\r\n", uid, len(raw), raw)
				}
				_, _ = conn.Write([]byte(tag + " OK FETCH completed\r\n"))
			}
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
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split smtp listener address: %v", err)
	}
	return host, port
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
