package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
