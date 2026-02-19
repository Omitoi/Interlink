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
// Returns all "accepted" connections to the logged in user and for each peer:
// name, picture, latest message and unread count
func chatSummaryHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		// CTEs for clarity.
		// 1) accepted = all peer ids
		// 2) chat_pairs = poosible chats row for this peer (can be NULL, if no messages)
		// 3) unreads = count undread messages (m.is_read = false) sent to me by this peer
		// Finally add to user details.
		const q = `
WITH accepted AS (
  SELECT CASE WHEN c.user_id = $1 THEN c.target_user_id ELSE c.user_id END AS peer_id
  FROM connections c
  WHERE c.status = 'accepted' AND (c.user_id = $1 OR c.target_user_id = $1)
),
chat_pairs AS (
  SELECT a.peer_id,
         ch.id AS chat_id,
         ch.last_message_at
  FROM accepted a
  LEFT JOIN chats ch
    ON ch.user1_id = LEAST($1::int, a.peer_id)
   AND ch.user2_id = GREATEST($1::int, a.peer_id)
),
unreads AS (
  SELECT cp.peer_id,
         COALESCE(SUM(CASE WHEN m.is_read = FALSE AND m.sender_id = cp.peer_id THEN 1 ELSE 0 END), 0) AS unread_count
  FROM chat_pairs cp
  LEFT JOIN messages m ON m.chat_id = cp.chat_id
  GROUP BY cp.peer_id
)
SELECT
  u.id AS user_id,
  COALESCE(p.display_name, CONCAT('User ', u.id::text)) AS display_name,
  p.profile_picture_file,
  cp.last_message_at,
  COALESCE(uR.unread_count, 0) AS unread_count
FROM accepted a
JOIN users u            ON u.id = a.peer_id
LEFT JOIN profiles p    ON p.user_id = u.id
LEFT JOIN chat_pairs cp ON cp.peer_id = a.peer_id
LEFT JOIN unreads uR    ON uR.peer_id = a.peer_id
ORDER BY COALESCE(cp.last_message_at, to_timestamp(0)) DESC, u.id ASC
;`

		rows, err := db.Query(q, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to query chat summary")
			return
		}
		defer rows.Close()

		summaries := make([]ChatPeerSummary, 0, 32)
		for rows.Next() {
			var s ChatPeerSummary
			var name string
			var pic *string
			var last *time.Time
			var unread int

			if err := rows.Scan(&s.UserID, &name, &pic, &last, &unread); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to scan chat summary")
				return
			}
			s.UserName = name
			s.ProfilePicture = pic
			s.LastMessageAt = last
			s.UnreadMessages = unread

			IsOnline, errIsOnline := isOnlineNow(r.Context(), db, s.UserID)
			if errIsOnline != nil {
				s.IsOnline = false
			} else {
				s.IsOnline = IsOnline
			}
			summaries = append(summaries, s)
		}
		if err := rows.Err(); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read chat summary rows")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(summaries)
	})
}

// POST /chats/read?peer_id=123
// For receiving the ack from frontend that a message has been read
func chatsMarkReadHandler(db *sql.DB) http.HandlerFunc {
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

		// Resolve chat_id for this pair
		var chatID int
		err = db.QueryRow(`
			SELECT id
			FROM chats
			WHERE user1_id = LEAST($1::int, $2::int)
			  AND user2_id = GREATEST($1::int, $2::int)
			LIMIT 1
		`, userID, peerID).Scan(&chatID)
		if err == sql.ErrNoRows {
			// No chat -> nothing to mark
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}

		markChatAsRead(db, chatID, userID, peerID)
		w.WriteHeader(http.StatusNoContent)
	})
}
