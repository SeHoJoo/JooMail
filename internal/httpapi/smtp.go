package httpapi

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

const smtpCommandTimeout = 10 * time.Second

var newSMTPTLSConfig = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

type sendRequest struct {
	To       []string `json:"to"`
	Cc       []string `json:"cc"`
	Bcc      []string `json:"bcc"`
	Subject  string   `json:"subject"`
	TextBody string   `json:"textBody"`
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	var request sendRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid send request")
		return
	}
	if len(request.To) == 0 || strings.TrimSpace(request.Subject) == "" {
		writeError(w, http.StatusBadRequest, "to and subject are required")
		return
	}
	if err := s.sendMail(auth.credential, request); err != nil {
		writeError(w, http.StatusBadGateway, "failed to send message")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) sendMail(credential storedCredential, request sendRequest) error {
	if s.config.SMTPHost == "" || s.config.SMTPPort == "" || s.config.SMTPUserFormat != "localpart" {
		return errors.New("smtp unavailable")
	}
	message := formatOutgoingMessage(credential.Email, request)
	recipients := append([]string{}, request.To...)
	recipients = append(recipients, request.Cc...)
	recipients = append(recipients, request.Bcc...)
	for i, recipient := range recipients {
		recipients[i] = strings.TrimSpace(recipient)
		if recipients[i] == "" {
			return errors.New("empty recipient")
		}
	}

	address := net.JoinHostPort(s.config.SMTPHost, s.config.SMTPPort)
	dialer := &net.Dialer{Timeout: smtpCommandTimeout}
	var conn net.Conn
	var err error
	if smtpImplicitTLS(s.config) {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, newSMTPTLSConfig(s.config.SMTPHost))
	} else {
		conn, err = dialer.Dial("tcp", address)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpCommandTimeout))
	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return err
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return err
	}
	if s.config.SMTPStartTLS && !smtpImplicitTLS(s.config) {
		if err := client.StartTLS(newSMTPTLSConfig(s.config.SMTPHost)); err != nil {
			return err
		}
	}
	auth := smtp.PlainAuth("", credential.IMAPUsername, credential.Password, s.config.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(credential.Email); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := client.Quit(); err != nil {
		return err
	}
	return s.appendSentMessage(credential, message)
}

func (s *Server) appendSentMessage(credential storedCredential, message string) error {
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.appendSentMessage(message)
}

func smtpImplicitTLS(config Config) bool {
	return config.SMTPTLS || config.SMTPPort == "465"
}

func formatOutgoingMessage(from string, request sendRequest) string {
	headers := []string{
		"From: " + from,
		"To: " + strings.Join(request.To, ", "),
	}
	if len(request.Cc) > 0 {
		headers = append(headers, "Cc: "+strings.Join(request.Cc, ", "))
	}
	headers = append(headers,
		"Subject: "+mime.QEncoding.Encode("UTF-8", strings.ReplaceAll(request.Subject, "\r\n", " ")),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	)
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + request.TextBody + "\r\n"
}
