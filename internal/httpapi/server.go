package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Server struct {
	store *Store
	mux   *http.ServeMux
}

func NewServer(store *Store) http.Handler {
	server := &Server{store: store, mux: http.NewServeMux()}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/accounts", s.handleAccounts)
	s.mux.HandleFunc("GET /api/accounts/{accountID}/mailboxes/{mailboxID}/messages", s.handleMessageSummaries)
	s.mux.HandleFunc("GET /api/messages/{messageID}", s.handleMessage)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAccounts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"accounts": s.store.Accounts()})
}

func (s *Server) handleMessageSummaries(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("accountID"))
	mailboxID := strings.TrimSpace(r.PathValue("mailboxID"))
	if accountID == "" || mailboxID == "" {
		writeError(w, http.StatusBadRequest, "accountId and mailboxId are required")
		return
	}

	messages := s.store.MessageSummaries(accountID, mailboxID, r.URL.Query().Get("q"))
	writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
}

func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	messageID := strings.TrimSpace(r.PathValue("messageID"))
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "messageId is required")
		return
	}

	message, err := s.store.Message(messageID)
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
