package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	recorder := request(t, "/api/health")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body map[string]string
	decode(t, recorder, &body)
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
	}
}

func TestAccounts(t *testing.T) {
	recorder := request(t, "/api/accounts")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Accounts []Account `json:"accounts"`
	}
	decode(t, recorder, &body)
	if len(body.Accounts) != 2 {
		t.Fatalf("account count = %d, want 2", len(body.Accounts))
	}
	if body.Accounts[0].Email != "jooseho@gmail.com" {
		t.Fatalf("first account email = %q", body.Accounts[0].Email)
	}
}

func TestMailboxMessages(t *testing.T) {
	recorder := request(t, "/api/accounts/personal/mailboxes/inbox/messages?q=mime")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Messages []MessageSummary `json:"messages"`
	}
	decode(t, recorder, &body)
	if len(body.Messages) != 2 {
		t.Fatalf("message count = %d, want 2", len(body.Messages))
	}
	for _, message := range body.Messages {
		if message.AccountID != "personal" || message.MailboxID != "inbox" {
			t.Fatalf("unexpected message scope: %#v", message)
		}
	}
}

func TestMessage(t *testing.T) {
	recorder := request(t, "/api/messages/m1")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	var body struct {
		Message Message `json:"message"`
	}
	decode(t, recorder, &body)
	if body.Message.ID != "m1" {
		t.Fatalf("message id = %q, want m1", body.Message.ID)
	}
	if len(body.Message.TextBody) == 0 {
		t.Fatal("message textBody is empty")
	}
}

func TestMessageNotFound(t *testing.T) {
	recorder := request(t, "/api/messages/missing")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestLoginRejectsMalformedRequestBody(t *testing.T) {
	server := NewServerWithConfig(MockStore(), testConfig("127.0.0.1", "1"))
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{"))

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLoginRejectsMalformedEmail(t *testing.T) {
	server := NewServerWithConfig(MockStore(), testConfig("127.0.0.1", "1"))
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"jooseho","password":"secret"}`))

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestLoginAuthenticatesAgainstIMAPAndSetsSessionCookie(t *testing.T) {
	host, port := startFakeIMAPServer(t, func(line string) string {
		if !strings.Contains(line, `LOGIN "jooseho" "correct-password"`) {
			t.Fatalf("login command = %q", line)
		}
		return "OK LOGIN completed"
	})
	server := NewServerWithConfig(MockStore(), testConfig(host, port))
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"jooseho@good-night.co.kr","password":"correct-password"}`))

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
	cookie := recorder.Result().Cookies()[0]
	if cookie.Name != "joomail_session" {
		t.Fatalf("cookie name = %q, want joomail_session", cookie.Name)
	}
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" {
		t.Fatalf("cookie security attributes = %#v", cookie)
	}
	if _, err := verifySessionToken(cookie.Value, "test-session-secret"); err != nil {
		t.Fatalf("verify session token: %v", err)
	}
}

func TestLoginReturnsUnauthorizedOnIMAPFailure(t *testing.T) {
	host, port := startFakeIMAPServer(t, func(line string) string {
		if !strings.Contains(line, "LOGIN") {
			t.Fatalf("login command = %q", line)
		}
		return "NO LOGIN failed"
	})
	server := NewServerWithConfig(MockStore(), testConfig(host, port))
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewBufferString(`{"email":"jooseho@good-night.co.kr","password":"wrong"}`))

	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
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
	var body map[string]string
	decode(t, recorder, &body)
	if body["status"] != "ok" {
		t.Fatalf("status body = %q, want ok", body["status"])
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

func request(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	server := NewServer(MockStore())
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
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

func testConfig(host string, port string) Config {
	return Config{
		IMAPHost:       host,
		IMAPPort:       port,
		IMAPTLS:        false,
		IMAPUserFormat: "localpart",
		SessionSecret:  "test-session-secret",
	}
}

func startFakeIMAPServer(t *testing.T, loginResponse func(line string) string) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake imap: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	done := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("fake imap server did not finish")
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
		if _, err := conn.Write([]byte("* OK fake IMAP ready\r\n")); err != nil {
			return
		}
		loginLine, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		tag := strings.Fields(loginLine)[0]
		if _, err := conn.Write([]byte(tag + " " + loginResponse(strings.TrimSpace(loginLine)) + "\r\n")); err != nil {
			return
		}
		logoutLine, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		logoutTag := strings.Fields(logoutLine)[0]
		_, _ = conn.Write([]byte("* BYE logging out\r\n" + logoutTag + " OK LOGOUT completed\r\n"))
	}()

	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	return host, port
}
