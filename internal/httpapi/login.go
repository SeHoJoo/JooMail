package httpapi

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode"
)

const imapLoginTimeout = 10 * time.Second

var errInvalidCredentials = errors.New("invalid credentials")

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login request")
		return
	}

	email := strings.TrimSpace(request.Email)
	if email == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	localPart, domain, ok := splitLoginEmail(email)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid email address")
		return
	}
	if !s.loginDomainAllowed(domain) {
		writeError(w, http.StatusUnauthorized, "이메일 또는 비밀번호가 올바르지 않습니다")
		return
	}

	if s.config.IMAPHost == "" || s.config.IMAPPort == "" || s.config.IMAPUserFormat == "" || s.config.SessionSecret == "" {
		writeError(w, http.StatusInternalServerError, "login is unavailable")
		return
	}
	if s.config.IMAPUserFormat != "localpart" {
		writeError(w, http.StatusInternalServerError, "login is unavailable")
		return
	}

	if err := verifyIMAPCredentials(s.config, localPart, request.Password); errors.Is(err, errInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "이메일 또는 비밀번호가 올바르지 않습니다")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "login is unavailable")
		return
	}

	token, payload, err := newSessionToken(email, request.Remember, time.Now(), s.config.SessionSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login is unavailable")
		return
	}
	credentials, err := newCredentialStore(s.config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "login is unavailable")
		return
	}
	if err := credentials.Save(payload.SessionID, storedCredential{
		Email:        email,
		IMAPUsername: localPart,
		Password:     request.Password,
		ExpiresAt:    payload.ExpiresAt,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "login is unavailable")
		return
	}
	cookie := &http.Cookie{
		Name:     "joomail_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
	if request.Remember {
		cookie.MaxAge = int(payload.ExpiresAt.Sub(payload.IssuedAt).Seconds())
	}
	http.SetCookie(w, cookie)

	writeJSON(w, http.StatusOK, map[string]any{"account": s.accountForLogin(email, localPart)})
}

func splitLoginEmail(email string) (string, string, bool) {
	localPart, domain, ok := strings.Cut(email, "@")
	return localPart, strings.ToLower(domain), ok && localPart != "" && domain != "" && !strings.Contains(domain, "@")
}

func (s *Server) loginDomainAllowed(domain string) bool {
	allowedDomain := configuredLoginDomain(s.config)
	return allowedDomain == "" || strings.EqualFold(domain, allowedDomain)
}

func configuredLoginDomain(config Config) string {
	if config.LoginDomain != "" {
		return strings.ToLower(strings.TrimSpace(config.LoginDomain))
	}
	host := strings.ToLower(strings.TrimSpace(config.IMAPHost))
	for _, prefix := range []string{"mail.", "imap.", "dovecot."} {
		if strings.HasPrefix(host, prefix) {
			return strings.TrimPrefix(host, prefix)
		}
	}
	return ""
}

func (s *Server) accountForLogin(email string, localPart string) Account {
	return Account{
		ID:        email,
		Email:     email,
		Label:     localPart,
		Initials:  firstInitial(localPart),
		Unread:    0,
		Storage:   "",
		Status:    "available",
		Mailboxes: []Mailbox{},
	}
}

func firstInitial(value string) string {
	for _, r := range value {
		return string(unicode.ToUpper(r))
	}
	return ""
}

func verifyIMAPCredentials(config Config, username string, password string) error {
	conn, err := dialIMAP(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(imapLoginTimeout)); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	if _, err := reader.ReadString('\n'); err != nil {
		return err
	}

	loginTag := "A001"
	if _, err := fmt.Fprintf(conn, "%s LOGIN %s %s\r\n", loginTag, quoteIMAPString(username), quoteIMAPString(password)); err != nil {
		return err
	}

	status, err := readTaggedStatus(reader, loginTag)
	if err != nil {
		return err
	}

	logoutTag := "A002"
	_, _ = fmt.Fprintf(conn, "%s LOGOUT\r\n", logoutTag)
	_, _ = readTaggedStatus(reader, logoutTag)

	switch status {
	case "OK":
		return nil
	case "NO", "BAD":
		return errInvalidCredentials
	default:
		return fmt.Errorf("unexpected imap status %q", status)
	}
}

func dialIMAP(config Config) (net.Conn, error) {
	address := net.JoinHostPort(config.IMAPHost, config.IMAPPort)
	dialer := &net.Dialer{Timeout: imapLoginTimeout}
	if config.IMAPTLS {
		return tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
			ServerName: config.IMAPHost,
			MinVersion: tls.VersionTLS12,
		})
	}
	return dialer.Dial("tcp", address)
}

func quoteIMAPString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func readTaggedStatus(reader *bufio.Reader, tag string) (string, error) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, tag+" ") && !strings.HasPrefix(line, tag+"\t") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return "", fmt.Errorf("invalid tagged imap response")
		}
		return strings.ToUpper(fields[1]), nil
	}
}
