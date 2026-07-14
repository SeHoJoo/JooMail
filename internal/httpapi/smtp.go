package httpapi

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

const (
	smtpCommandTimeout  = 10 * time.Second
	sendRequestMaxBytes = 32 << 20
)

var newSMTPTLSConfig = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

type sendRequest struct {
	FromName    string               `json:"fromName"`
	To          []string             `json:"to"`
	Cc          []string             `json:"cc"`
	Bcc         []string             `json:"bcc"`
	Subject     string               `json:"subject"`
	TextBody    string               `json:"textBody"`
	Attachments []outgoingAttachment `json:"-"`
}

type outgoingAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, sendRequestMaxBytes)
	var request sendRequest
	if err := parseSendRequest(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid send request")
		return
	}
	if strings.TrimSpace(request.Subject) == "" {
		writeError(w, http.StatusBadRequest, "to and subject are required")
		return
	}
	if err := validateRecipients(&request); err != nil {
		if errors.Is(err, errMissingRecipient) {
			writeError(w, http.StatusBadRequest, "to and subject are required")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid recipient")
		return
	}
	sentCopyStored, err := s.sendMail(auth.credential, request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to send message")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "sent", "sentCopyStored": sentCopyStored})
}

func (s *Server) handleSaveDraft(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, sendRequestMaxBytes)
	var request sendRequest
	if err := parseSendRequest(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid draft request")
		return
	}
	if err := s.saveDraft(auth.credential, request); err != nil {
		writeError(w, http.StatusBadGateway, "failed to save draft")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func parseSendRequest(r *http.Request, request *sendRequest) error {
	contentType := r.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if strings.EqualFold(mediaType, "multipart/form-data") {
		return parseMultipartSendRequest(r, request)
	}
	return json.NewDecoder(r.Body).Decode(request)
}

func parseMultipartSendRequest(r *http.Request, request *sendRequest) error {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return err
	}
	request.To = formRecipientList(r, "to")
	request.Cc = formRecipientList(r, "cc")
	request.Bcc = formRecipientList(r, "bcc")
	request.FromName = r.FormValue("fromName")
	request.Subject = r.FormValue("subject")
	request.TextBody = r.FormValue("textBody")
	if r.MultipartForm == nil {
		return nil
	}
	for _, fileHeader := range r.MultipartForm.File["attachments"] {
		file, err := fileHeader.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(file)
		_ = file.Close()
		if readErr != nil {
			return readErr
		}
		contentType := fileHeader.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		request.Attachments = append(request.Attachments, outgoingAttachment{
			Filename:    fileHeader.Filename,
			ContentType: contentType,
			Data:        data,
		})
	}
	return nil
}

func formRecipientList(r *http.Request, key string) []string {
	value := strings.TrimSpace(r.FormValue(key))
	if value == "" {
		return nil
	}
	var out []string
	if json.Unmarshal([]byte(value), &out) == nil {
		return compactRecipients(out)
	}
	return compactRecipients(strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	}))
}

func compactRecipients(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

var errMissingRecipient = errors.New("missing recipient")

func validateRecipients(request *sendRequest) error {
	request.To = compactRecipients(request.To)
	request.Cc = compactRecipients(request.Cc)
	request.Bcc = compactRecipients(request.Bcc)
	if len(request.To) == 0 {
		return errMissingRecipient
	}
	recipients := append([]string{}, request.To...)
	recipients = append(recipients, request.Cc...)
	recipients = append(recipients, request.Bcc...)
	for _, recipient := range recipients {
		if _, err := mail.ParseAddress(recipient); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) sendMail(credential storedCredential, request sendRequest) (bool, error) {
	if s.config.SMTPHost == "" || s.config.SMTPPort == "" || s.config.SMTPUserFormat != "localpart" {
		return false, errors.New("smtp unavailable")
	}
	message := formatOutgoingMessage(credential.Email, request)
	recipients := append([]string{}, request.To...)
	recipients = append(recipients, request.Cc...)
	recipients = append(recipients, request.Bcc...)

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
		return false, err
	}
	_ = conn.SetDeadline(time.Now().Add(smtpCommandTimeout))
	client, err := smtp.NewClient(conn, s.config.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return false, err
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return false, err
	}
	if s.config.SMTPStartTLS && !smtpImplicitTLS(s.config) {
		if err := client.StartTLS(newSMTPTLSConfig(s.config.SMTPHost)); err != nil {
			return false, err
		}
	}
	auth := smtp.PlainAuth("", credential.IMAPUsername, credential.Password, s.config.SMTPHost)
	if err := client.Auth(auth); err != nil {
		return false, err
	}
	if err := client.Mail(credential.Email); err != nil {
		return false, err
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient); err != nil {
			return false, err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return false, err
	}
	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return false, err
	}
	if err := writer.Close(); err != nil {
		return false, err
	}
	_ = client.Quit()
	if err := s.appendSentMessage(credential, message); err != nil {
		return false, nil
	}
	return true, nil
}

func (s *Server) appendSentMessage(credential storedCredential, message string) error {
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.appendSentMessage(message)
}

func (s *Server) saveDraft(credential storedCredential, request sendRequest) error {
	client, err := openIMAPSession(s.config, credential)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.appendDraftMessage(formatOutgoingMessage(credential.Email, request))
}

func smtpImplicitTLS(config Config) bool {
	return config.SMTPTLS || config.SMTPPort == "465"
}

func formatOutgoingMessage(from string, request sendRequest) string {
	headers := []string{
		"From: " + formatFromHeader(from, request.FromName),
		"To: " + strings.Join(request.To, ", "),
		"Date: " + time.Now().Format(time.RFC1123Z),
	}
	if len(request.Cc) > 0 {
		headers = append(headers, "Cc: "+strings.Join(request.Cc, ", "))
	}
	headers = append(headers,
		"Subject: "+mime.QEncoding.Encode("UTF-8", strings.ReplaceAll(request.Subject, "\r\n", " ")),
		"MIME-Version: 1.0",
	)
	if len(request.Attachments) > 0 {
		return formatMultipartOutgoingMessage(headers, request)
	}
	headers = append(headers,
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	)
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + request.TextBody + "\r\n"
}

func formatFromHeader(email string, name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "\r", " "), "\n", " "))
	if name == "" {
		return email
	}
	return mime.QEncoding.Encode("UTF-8", name) + " <" + email + ">"
}

func formatMultipartOutgoingMessage(headers []string, request sendRequest) string {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	headers = append(headers, `Content-Type: multipart/mixed; boundary="`+writer.Boundary()+`"`)
	textHeader := textproto.MIMEHeader{}
	textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
	textHeader.Set("Content-Transfer-Encoding", "8bit")
	textPart, _ := writer.CreatePart(textHeader)
	_, _ = textPart.Write([]byte(request.TextBody + "\r\n"))
	for _, attachment := range request.Attachments {
		partHeader := textproto.MIMEHeader{}
		contentType := attachment.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		partHeader.Set("Content-Type", mime.FormatMediaType(contentType, map[string]string{"name": attachment.Filename}))
		partHeader.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Filename}))
		partHeader.Set("Content-Transfer-Encoding", "base64")
		part, _ := writer.CreatePart(partHeader)
		_, _ = part.Write([]byte(wrapBase64(attachment.Data)))
	}
	_ = writer.Close()
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body.String()
}

func wrapBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	if encoded == "" {
		return "\r\n"
	}
	var builder strings.Builder
	for len(encoded) > 76 {
		builder.WriteString(encoded[:76])
		builder.WriteString("\r\n")
		encoded = encoded[76:]
	}
	builder.WriteString(encoded)
	builder.WriteString("\r\n")
	return builder.String()
}
