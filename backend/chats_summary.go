package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

// ChatPeerSummary represents a summary of a chat peer with recent activity
type ChatPeerSummary struct {
	UserID         int        `json:"userId"`
	UserName       string     `json:"userName"`
	ProfilePicture *string    `json:"profilePicture,omitempty"`
	LastMessageAt  *time.Time `json:"lastMessageAt,omitempty"`
	UnreadMessages int        `json:"unreadMessages"`
	IsOnline       bool       `json:"isOnline,omitempty"`
}

// GET /chat/summary
// Returns all "accepted" connections to the logged in user with per-peer details.
func chatSummaryHandler(db *sql.DB) http.HandlerFunc {
	repo := NewChatRepository(db)
	svc := NewChatService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		summaries, err := svc.GetSummaries(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to query chat summary")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summaries)
	})
}

// POST /chats/read?peer_id=123
// Receives a read-acknowledgement from the frontend for a given peer's messages.
func chatsMarkReadHandler(db *sql.DB) http.HandlerFunc {
	repo := NewChatRepository(db)
	svc := NewChatService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		userID := r.Context().Value(userIDKey).(int)

		peerStr := r.URL.Query().Get("peer_id")
		peerID, err := strconv.Atoi(peerStr)
		if err != nil || peerID <= 0 {
			writeError(w, http.StatusBadRequest, "bad_peer_id")
			return
		}

		if err := svc.MarkRead(r.Context(), userID, peerID); err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})
}
