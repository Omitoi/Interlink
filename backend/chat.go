package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

// ChatMessage represents a chat message with metadata
type ChatMessage struct {
	ID     int64     `json:"id"`   // DB message id
	Type   string    `json:"type"` // "message"
	ChatID int       `json:"chat_id"`
	From   int       `json:"from"`
	To     int       `json:"to,omitempty"`
	Body   string    `json:"body,omitempty"`
	Ts     time.Time `json:"ts"` // created_at
}

// ServerEvent represents a server-sent event
type ServerEvent struct {
	Type string `json:"type"` // "message" | "typing" | "info" | "error"
	From int    `json:"from,omitempty"`
	Data any    `json:"data,omitempty"`
}

// Client represents a WebSocket client connection
type Client struct {
	userID int
	conn   *websocket.Conn
	send   chan ServerEvent
	db     *sql.DB
}

// Hub manages WebSocket client connections
type Hub struct {
	clientsByUser map[int]map[*Client]bool
	mu            sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		clientsByUser: make(map[int]map[*Client]bool),
	}
}

func (h *Hub) register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clientsByUser[c.userID] == nil {
		h.clientsByUser[c.userID] = make(map[*Client]bool)
	}
	h.clientsByUser[c.userID][c] = true
}

func (h *Hub) unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if peers, ok := h.clientsByUser[c.userID]; ok {
		delete(peers, c)
		if len(peers) == 0 {
			delete(h.clientsByUser, c.userID)
		}
	}
}

func (h *Hub) sendToUser(userID int, evt ServerEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if peers, ok := h.clientsByUser[userID]; ok {
		for c := range peers {
			select {
			case c.send <- evt:
			default:
				// Drop message if user's buffer is full
			}
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// For development: allow Vite dev origin ws://localhost:5173
	CheckOrigin: func(r *http.Request) bool { return true },
}

// global hub
var chatHub = newHub()

func wsChatHandler(db *sql.DB) http.HandlerFunc {
	// We'll authenticate using the existing JWT middleware logic inline here
	return func(w http.ResponseWriter, r *http.Request) {
		// Reuse the authenticate() behavior inline:
		userID, ok := getUserIDFromRequest(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS upgrade error for user %d: %v", userID, err)
			return
		}

		client := &Client{
			userID: userID,
			conn:   conn,
			send:   make(chan ServerEvent, 16),
			db:     db,
		}
		chatHub.register(client)

		// Announce connection to this client
		client.send <- ServerEvent{Type: "info", Data: "connected"}

		// Start writer
		go clientWriter(client)
		// Start reader (blocks)
		clientReader(client)
	}
}

// Extract user ID from Authorization header using the existing jwtSecret
// This mirrors the authenticate() logic, but returns (id,ok) instead of wrapping a handler.
func getUserIDFromBearer(r *http.Request) (int, bool) {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		return 0, false
	}
	tokenStr := auth[7:]
	id, ok := parseUserIDFromJWT(tokenStr)
	return id, ok
}

// replace getUserIDFromBearer with:
func getUserIDFromRequest(r *http.Request) (int, bool) {
	// Try Authorization header first
	if id, ok := getUserIDFromBearer(r); ok {
		return id, true
	}
	// Fallback: token query param for WS (browsers can't set headers)
	q := r.URL.Query().Get("token")
	if q != "" {
		return parseUserIDFromJWT(q)
	}
	return 0, false
}

func parseUserIDFromJWT(tokenStr string) (int, bool) {
	claims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, false
	}

	// jwt.MapClaims stores numbers as float64 by default
	fv, ok := claims["user_id"].(float64)
	if !ok {
		return 0, false
	}
	return int(fv), true
}

// tiny helper to use the existing jwtSecret without reimport noise
func jwtParse(s string) (*jwt.Token, error) {
	return jwt.Parse(s, func(token *jwt.Token) (any, error) { return jwtSecret, nil })
}

func clientReader(c *Client) {
	defer func() {
		chatHub.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg ChatMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			c.send <- ServerEvent{Type: "error", Data: "invalid message format"}
			continue
		}

		switch msg.Type {
		case "message":
			// 1) Save to database
			id, chatID, ts, err := saveChatMsg(c.db, c.userID, msg.To, msg.Body)
			if err != nil {
				c.send <- ServerEvent{Type: "error", Data: "cannot send message"}
				continue
			}

			outMsg := ChatMessage{
				ID:     id,
				Type:   "message",
				ChatID: chatID,
				From:   c.userID,
				To:     msg.To,
				Body:   msg.Body,
				Ts:     ts,
			}
			// minimal relay: send to recipient and echo back to sender
			out := ServerEvent{
				Type: "message",
				From: c.userID,
				Data: outMsg,
			}

			log.Printf("[CHAT DEBUG] Sending message to recipient %d", msg.To)
			chatHub.sendToUser(msg.To, out)
			log.Printf("[CHAT DEBUG] Echoing message back to sender %d", c.userID)
			chatHub.sendToUser(c.userID, out) // echo (so sender UI updates instantly)

		case "typing":
			log.Printf("[CHAT DEBUG] Processing typing indicator from %d to %d", c.userID, msg.To)
			// notify recipient that sender is typing
			chatHub.sendToUser(msg.To, ServerEvent{Type: "typing", From: c.userID})

		default:
			log.Printf("[CHAT DEBUG] Unknown message type from %d: %s", c.userID, msg.Type)
			c.send <- ServerEvent{Type: "error", Data: "unknown message type"}
		}
	}
}

func clientWriter(c *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case evt, ok := <-c.send:
			if !ok {
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteJSON(evt); err != nil {
				return
			}
		case <-ticker.C:
			// ping to keep the connection alive
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Helper function for saving the message history to database
func saveChatMsg(db *sql.DB, fromUserID int, toUserID int, content string) (int64, int, time.Time, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	// 1) Make sure that the connection status is 'accepted'
	var ok int
	err = tx.QueryRow(`
		SELECT 1
		FROM connections
		WHERE status = 'accepted'
			AND ((user_id = $1 AND target_user_id = $2) OR(user_id = $2 AND target_user_id = $1))
		LIMIT 1
	`, fromUserID, toUserID).Scan(&ok)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, time.Time{}, fmt.Errorf("no accepted connection")
		}
		return 0, 0, time.Time{}, err
	}

	// 2) Fetch or create a chat id
	var chatID int
	err = tx.QueryRow(`
		SELECT id
		FROM chats
		WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
		LIMIT 1
	`, fromUserID, toUserID).Scan(&chatID)
	if err == sql.ErrNoRows {
		// Create
		err = tx.QueryRow(`
			INSERT INTO chats (user1_id, user2_id)
			VALUES (LEAST($1::int, $2::int), GREATEST($1::int, $2::int))
			ON CONFLICT (user1_id, user2_id) DO NOTHING
			RETURNING id
		`, fromUserID, toUserID).Scan(&chatID)
		if err == sql.ErrNoRows {
			// Race: someone else created first -> refetch
			err = tx.QueryRow(`
				SELECT id
				FROM chats
				WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
				LIMIT 1
			`, fromUserID, toUserID).Scan(&chatID)
		}
	}
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	// 3) Add message
	var msgID int64
	var createdAt time.Time
	err = tx.QueryRow(`
		INSERT INTO messages (chat_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, chatID, fromUserID, content).Scan(&msgID, &createdAt)
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	// 4) Update last_message_at and unread to the peer
	_, err = tx.Exec(`
		UPDATE chats c
		SET last_message_at = $3,
			unread_for_user1 = CASE WHEN $2 = c.user2_id THEN TRUE ELSE unread_for_user1 END,
			unread_for_user2 = CASE WHEN $2 = c.user1_id THEN TRUE ELSE unread_for_user2 END
		WHERE c.id = $1
	`, chatID, fromUserID, createdAt)
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	return msgID, chatID, createdAt, nil
}

func getChatMessages(db *sql.DB, userID int, otherUserID int, limit int, before *time.Time) ([]ChatMessage, error) {
	// 1) Resolve chat id
	var chatID int
	err := db.QueryRow(`
		SELECT id
		FROM chats
		WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
		LIMIT 1
	`, userID, otherUserID).Scan(&chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []ChatMessage{}, nil
		}
		return nil, err
	}

	// 2) Fetch messages
	q := `
		SELECT id, sender_id, content, created_at
		FROM messages
		WHERE chat_id = $1
			AND ($2::timestamptz IS NULL OR created_at < $2)
			ORDER BY created_at DESC
		LIMIT $3`

	var rows *sql.Rows
	if before != nil {
		rows, err = db.Query(q, chatID, *before, limit)
	} else {
		rows, err = db.Query(q, chatID, nil, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgs := make([]ChatMessage, 0, limit)
	for rows.Next() {
		var msgID int64
		var senderID int
		var body string
		var createdAt time.Time
		if err := rows.Scan(&msgID, &senderID, &body, &createdAt); err != nil {
			return nil, err
		}

		msgs = append(msgs, ChatMessage{
			ID:     msgID,
			Type:   "message",
			ChatID: chatID,
			From:   senderID,
			Body:   body,
			Ts:     createdAt,
		})
	}

	// Check for errors after the last iteration
	if err := rows.Err(); err != nil {
		// Don't mark as read if the query failed
		return nil, err
	}

	// 3) Set all messages from the other user as read and clear unread flag for this user
	_, _ = db.Exec(`
		UPDATE messages
		SET is_read = TRUE
		WHERE chat_id = $1 AND sender_id <> $2 AND is_read IS FALSE
	`, chatID, userID)

	_, _ = db.Exec(`
		UPDATE chats c
		SET unread_for_user1 = CASE WHEN $2 = c.user1_id THEN FALSE ELSE unread_for_user1 END,
			unread_for_user2 = CASE WHEN $2 = c.user2_id THEN FALSE ELSE unread_for_user2 END
		WHERE c.id = $1
	`, chatID, userID)

	return msgs, nil
}

// GET /chats/{otherUserId}/messages?limit=50&before=2025-09-16T08:00:00Z
func getChatHistoryHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// auth: same logic as in everywhere else
		userID, ok := getUserIDFromBearer(r)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "chats" || parts[2] != "messages" {
			http.NotFound(w, r)
			return
		}
		otherID, err := strconv.Atoi(parts[1])
		if err != nil {
			http.Error(w, "bad user id", http.StatusBadRequest)
			return
		}

		// query params
		limit := 50
		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
				limit = n
			}
		}
		var beforePtr *time.Time
		if s := r.URL.Query().Get("before"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				beforePtr = &t
			}
		}

		msgs, err := getChatMessages(db, userID, otherID, limit, beforePtr)
		if err != nil {
			http.Error(w, "failed to fetch messages", http.StatusInternalServerError)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(msgs)
	}
}
