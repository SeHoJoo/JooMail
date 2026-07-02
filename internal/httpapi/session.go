package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const sessionDuration = 24 * time.Hour
const rememberedSessionDuration = 30 * 24 * time.Hour

type sessionPayload struct {
	SessionID string    `json:"sessionId"`
	Email     string    `json:"email"`
	IssuedAt  time.Time `json:"issuedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Remember  bool      `json:"remember"`
}

func newSessionToken(email string, remember bool, issuedAt time.Time, secret string) (string, sessionPayload, error) {
	duration := sessionDuration
	if remember {
		duration = rememberedSessionDuration
	}
	sessionID, err := newSessionID()
	if err != nil {
		return "", sessionPayload{}, err
	}
	payload := sessionPayload{
		SessionID: sessionID,
		Email:     email,
		IssuedAt:  issuedAt.UTC(),
		ExpiresAt: issuedAt.Add(duration).UTC(),
		Remember:  remember,
	}
	if secret == "" {
		return "", payload, errors.New("session secret is required")
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", payload, err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	signature := signSessionPayload(encodedPayload, secret)
	return encodedPayload + "." + signature, payload, nil
}

func verifySessionToken(token string, secret string) (sessionPayload, error) {
	var payload sessionPayload
	if secret == "" {
		return payload, errors.New("session secret is required")
	}

	encodedPayload, encodedSignature, ok := strings.Cut(token, ".")
	if !ok || encodedPayload == "" || encodedSignature == "" {
		return payload, errors.New("invalid session token")
	}

	expectedSignature := signSessionPayload(encodedPayload, secret)
	if !hmac.Equal([]byte(encodedSignature), []byte(expectedSignature)) {
		return payload, errors.New("invalid session signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return payload, err
	}
	if time.Now().After(payload.ExpiresAt) {
		return payload, errors.New("session expired")
	}
	return payload, nil
}

func newSessionID() (string, error) {
	var bytes [32]byte
	if _, err := io.ReadFull(rand.Reader, bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func signSessionPayload(encodedPayload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
