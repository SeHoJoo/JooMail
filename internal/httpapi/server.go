package httpapi

import (
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
)

type Server struct {
	config Config
	mux    *http.ServeMux
}

func NewServer() http.Handler {
	return NewServerWithConfig(LoadConfig())
}

func NewServerWithConfig(config Config) http.Handler {
	server := &Server{config: config, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/login", s.handleLogin)
	s.mux.HandleFunc("POST /api/logout", s.handleLogout)
	s.mux.HandleFunc("GET /api/accounts", s.handleAccounts)
	s.mux.HandleFunc("POST /api/accounts", s.handleAddAccount)
	s.mux.HandleFunc("GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages", s.handleMessageSummaries)
	s.mux.HandleFunc("GET /api/messages/{messageID}", s.handleMessage)
	s.mux.HandleFunc("GET /api/messages/{messageID}/attachments/{attachmentID}", s.handleMessageAttachment)
	s.mux.HandleFunc("PATCH /api/messages/{messageID}/flagged", s.handleMessageFlagged)
	s.mux.HandleFunc("PATCH /api/messages/{messageID}/seen", s.handleMessageSeen)
	s.mux.HandleFunc("POST /api/messages/{messageID}/move", s.handleMessageMove)
	s.mux.HandleFunc("POST /api/send", s.handleSend)
	s.mux.HandleFunc("POST /api/drafts", s.handleSaveDraft)
	s.mux.HandleFunc("GET /api/accounts/{accountID}/rules", s.handleRules)
	s.mux.HandleFunc("PUT /api/accounts/{accountID}/rules", s.handleSaveRules)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	type result struct {
		index   int
		account Account
	}
	results := make(chan result, len(auth.credentials.Accounts))
	for index, credential := range auth.credentials.Accounts {
		go func(index int, credential storedCredential) {
			account := s.accountForSession(credential.Email, nil)
			account.Status = "unavailable"
			client, err := openIMAPSession(s.config, credential)
			if errors.Is(err, errInvalidCredentials) {
				account.Status = "reauth-required"
				results <- result{index, account}
				return
			}
			if err != nil {
				results <- result{index, account}
				return
			}
			defer client.Close()
			mailboxes, err := client.listMailboxes()
			if err == nil {
				account.Mailboxes = client.withUnreadCounts(mailboxes)
				account.Unread = totalUnread(account.Mailboxes)
				account.Status = "available"
			}
			results <- result{index, account}
		}(index, credential)
	}
	accounts := make([]Account, len(auth.credentials.Accounts))
	for range accounts {
		result := <-results
		accounts[result.index] = result.account
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accounts})
}

func (s *Server) handleAddAccount(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid account request")
		return
	}
	email := strings.TrimSpace(request.Email)
	localPart, domain, valid := splitLoginEmail(email)
	if !valid || request.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if !s.loginDomainAllowed(domain) || verifyIMAPCredentials(s.config, localPart, request.Password) != nil {
		writeError(w, http.StatusUnauthorized, "이메일 또는 비밀번호가 올바르지 않습니다")
		return
	}
	credential := storedCredential{Email: email, IMAPUsername: localPart, Password: request.Password, ExpiresAt: auth.payload.ExpiresAt}
	bundle := auth.credentials
	replaced := false
	for i := range bundle.Accounts {
		if equalEmail(bundle.Accounts[i].Email, email) {
			bundle.Accounts[i] = credential
			replaced = true
		}
	}
	if !replaced {
		bundle.Accounts = append(bundle.Accounts, credential)
	}
	store, err := newCredentialStore(s.config)
	if err != nil || store.SaveBundle(auth.payload.SessionID, bundle) != nil {
		writeError(w, http.StatusInternalServerError, "account is unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account": s.accountForLogin(email, localPart)})
}

func (s *Server) handleMessageSummaries(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	accountID := strings.TrimSpace(r.PathValue("accountID"))
	mailboxID := strings.TrimSpace(r.PathValue("mailboxID"))
	if accountID == "" || mailboxID == "" {
		writeError(w, http.StatusBadRequest, "accountId and mailboxId are required")
		return
	}
	credential, found := auth.credentialForAccount(accountID)
	if !found {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}

	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load messages")
		return
	}
	defer client.Close()
	scope, err := parseMessageSearchScope(r.URL.Query().Get("scope"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid search scope")
		return
	}
	messages, err := client.messageSummaries(accountID, mailboxID, r.URL.Query().Get("q"), scope)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load messages")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(r.PathValue("messageID"))
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "messageId is required")
		return
	}

	credential, found := auth.credentialForMessage(messageID)
	if !found {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load message")
		return
	}
	defer client.Close()
	message, err := client.message(auth.credential.Email, messageID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load message")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": message})
}

func (s *Server) handleMessageAttachment(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(r.PathValue("messageID"))
	attachmentID := strings.TrimSpace(r.PathValue("attachmentID"))
	if messageID == "" || attachmentID == "" {
		writeError(w, http.StatusBadRequest, "messageId and attachmentId are required")
		return
	}

	credential, found := auth.credentialForMessage(messageID)
	if !found {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load attachment")
		return
	}
	defer client.Close()
	attachment, err := client.messageAttachment(messageID, attachmentID)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "attachment not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load attachment")
		return
	}

	w.Header().Set("Content-Type", attachment.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Name}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(attachment.Data)
}

func (s *Server) handleMessageFlagged(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(r.PathValue("messageID"))
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "messageId is required")
		return
	}
	var request struct {
		Flagged bool `json:"flagged"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid flagged request")
		return
	}
	credential, found := auth.credentialForMessage(messageID)
	if !found {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to update message")
		return
	}
	defer client.Close()
	if err := client.setMessageFlagged(messageID, request.Flagged); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "failed to update message")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"flagged": request.Flagged})
}

func (s *Server) handleMessageSeen(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(r.PathValue("messageID"))
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "messageId is required")
		return
	}
	var request struct {
		Seen bool `json:"seen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid seen request")
		return
	}
	credential, found := auth.credentialForMessage(messageID)
	if !found {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to update message")
		return
	}
	defer client.Close()
	if err := client.setMessageSeen(messageID, request.Seen); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "failed to update message")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"seen": request.Seen})
}

func (s *Server) handleMessageMove(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	messageID := strings.TrimSpace(r.PathValue("messageID"))
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "messageId is required")
		return
	}
	var request struct {
		MailboxID string `json:"mailboxId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || strings.TrimSpace(request.MailboxID) == "" {
		writeError(w, http.StatusBadRequest, "invalid move request")
		return
	}
	credential, found := auth.credentialForMessage(messageID)
	if !found {
		writeError(w, http.StatusNotFound, "message not found")
		return
	}
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to move message")
		return
	}
	defer client.Close()
	if err := client.moveMessage(messageID, request.MailboxID); errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "message not found")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "failed to move message")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "moved"})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	if credentials, err := newCredentialStore(s.config); err == nil {
		_ = credentials.Delete(auth.payload.SessionID)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "joomail_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
