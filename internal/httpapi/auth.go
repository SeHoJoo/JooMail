package httpapi

import (
	"net/http"
	"strings"
)

type authenticatedRequest struct {
	payload    sessionPayload
	credential storedCredential
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
	credential, err := credentials.Load(payload.SessionID, payload.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return auth, false
	}
	auth.payload = payload
	auth.credential = credential
	return auth, true
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
