package httpapi

import (
	"net/http"
	"strings"
	"time"
)

type authenticatedRequest struct {
	payload     sessionPayload
	credentials credentialBundle
	credential  storedCredential // first account, retained for single-account route compatibility
}

func (s *Server) requireCredential(w http.ResponseWriter, r *http.Request) (authenticatedRequest, bool) {
	var auth authenticatedRequest
	cookie, err := r.Cookie("joomail_session")
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth, false
	}
	payload, err := verifySessionToken(cookie.Value, s.config.SessionSecret)
	if err != nil || payload.SessionID == "" || payload.Email == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth, false
	}
	credentials, err := newCredentialStore(s.config)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth, false
	}
	bundle, err := credentials.LoadBundle(payload.SessionID)
	if err != nil {
		_ = credentials.Delete(payload.SessionID)
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth, false
	}
	if len(bundle.Accounts) == 0 || time.Now().After(payload.ExpiresAt) {
		_ = credentials.Delete(payload.SessionID)
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth, false
	}
	auth.payload = payload
	auth.credentials = bundle
	auth.credential = bundle.Accounts[0]
	return auth, true
}

func (auth authenticatedRequest) credentialForAccount(accountID string) (storedCredential, bool) {
	for _, credential := range auth.credentials.Accounts {
		if equalEmail(credential.Email, accountID) && !time.Now().After(credential.ExpiresAt) {
			return credential, true
		}
	}
	return storedCredential{}, false
}

func (auth authenticatedRequest) onlyCredential() (storedCredential, bool) {
	if len(auth.credentials.Accounts) != 1 {
		return storedCredential{}, false
	}
	credential := auth.credentials.Accounts[0]
	return credential, !time.Now().After(credential.ExpiresAt)
}

func (auth authenticatedRequest) credentialForOutgoing(accountID string) (storedCredential, bool) {
	if strings.TrimSpace(accountID) == "" {
		return auth.onlyCredential()
	}
	return auth.credentialForAccount(accountID)
}

func (auth authenticatedRequest) credentialForMessage(messageID string) (storedCredential, bool) {
	accountID, _, _, _, legacy, err := decodeMessageReference(messageID)
	if err != nil || (legacy && len(auth.credentials.Accounts) != 1) {
		return storedCredential{}, false
	}
	if legacy {
		return auth.onlyCredential()
	}
	return auth.credentialForAccount(accountID)
}

func (s *Server) accountForSession(email string, mailboxes []Mailbox) Account {
	localPart, _, _ := splitLoginEmail(email)
	return Account{
		ID:        email,
		Email:     email,
		Label:     localPart,
		Initials:  firstInitial(localPart),
		Unread:    totalUnread(mailboxes),
		Storage:   "",
		Mailboxes: mailboxes,
	}
}

func totalUnread(mailboxes []Mailbox) int {
	total := 0
	for _, mailbox := range mailboxes {
		total += mailbox.Unread
		total += totalUnread(mailbox.Children)
	}
	return total
}

func equalEmail(a string, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
