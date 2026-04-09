package main

import (
	"context"
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
	ID     int64     `json:"id"`
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
	userID  int
	conn    *websocket.Conn
	send    chan ServerEvent
	chatSvc ChatService
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
	repo := NewChatRepository(db)
	svc := NewChatService(repo, db)

	// WebSocket upgrade hijacks the response, so we cannot use the authenticate() wrapper.
	// Auth is handled inline via getUserIDFromRequest.
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := getUserIDFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WS upgrade error for user %d: %v", userID, err)
			return
		}

		client := &Client{
			userID:  userID,
			conn:    conn,
			send:    make(chan ServerEvent, 16),
			chatSvc: svc,
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
func getUserIDFromBearer(r *http.Request) (int, bool) {
	auth := r.Header.Get("Authorization")
	if len(auth) < 8 || auth[:7] != "Bearer " {
		return 0, false
	}
	tokenStr := auth[7:]
	id, ok := parseUserIDFromJWT(tokenStr)
	return id, ok
}

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

// clientReader handles incoming WebSocket messages from a connected client.
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
			outMsg, err := c.chatSvc.SendMessage(context.Background(), c.userID, msg.To, msg.Body)
			if err != nil {
				c.send <- ServerEvent{Type: "error", Data: "cannot send message"}
				continue
			}

			out := ServerEvent{
				Type: "message",
				From: c.userID,
				Data: outMsg,
			}

			log.Printf("[CHAT DEBUG] Sending message to recipient %d", msg.To)
			chatHub.sendToUser(msg.To, out)
			log.Printf("[CHAT DEBUG] Echoing message back to sender %d", c.userID)
			chatHub.sendToUser(c.userID, out) // echo so sender UI updates instantly

		case "typing":
			log.Printf("[CHAT DEBUG] Processing typing indicator from %d to %d", c.userID, msg.To)
			chatHub.sendToUser(msg.To, ServerEvent{Type: "typing", From: c.userID})

		default:
			log.Printf("[CHAT DEBUG] Unknown message type from %d: %s", c.userID, msg.Type)
			c.send <- ServerEvent{Type: "error", Data: "unknown message type"}
		}
	}
}

// clientWriter pumps outgoing events to the WebSocket connection.
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

// GET /chats/{otherUserId}/messages?limit=50&before=2025-09-16T08:00:00Z
func getChatHistoryHandler(db *sql.DB) http.HandlerFunc {
	repo := NewChatRepository(db)
	svc := NewChatService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "chats" || parts[2] != "messages" {
			http.NotFound(w, r)
			return
		}
		otherID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_user_id")
			return
		}

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

		msgs, err := svc.GetHistory(r.Context(), userID, otherID, limit, beforePtr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed_to_fetch_messages")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(msgs)
	})
}

// Backward-compatibility wrappers called by tests that import these directly.
func saveChatMsg(ctx context.Context, db *sql.DB, fromUserID, toUserID int, content string) (int64, int, time.Time, error) {
	return NewChatRepository(db).SaveChatMsg(ctx, fromUserID, toUserID, content)
}

func getChatMessages(ctx context.Context, db *sql.DB, userID, otherUserID, limit int, before *time.Time) ([]ChatMessage, error) {
	repo := NewChatRepository(db)
	msgs, err := repo.GetChatMessages(ctx, userID, otherUserID, limit, before)
	if err != nil {
		return nil, err
	}
	chatID, err := repo.GetChatIDForPair(ctx, userID, otherUserID)
	if err == nil {
		_ = repo.MarkChatAsRead(chatID, userID, otherUserID)
	}
	return msgs, nil
}

func markChatAsRead(db *sql.DB, chatID, readerUserID, senderUserID int) error {
	return NewChatRepository(db).MarkChatAsRead(chatID, readerUserID, senderUserID)
}
