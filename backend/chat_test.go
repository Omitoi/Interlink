package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test saveChatMsg function
func TestSaveChatMsg(t *testing.T) {
	user1 := createTestUser(t, "savechat1@example.com", "password123")
	user2 := createTestUser(t, "savechat2@example.com", "password123")
	testProfile := getDefaultTestProfile()
	createTestProfile(t, user1, testProfile)
	createTestProfile(t, user2, testProfile)
	createConnection(t, user1.ID, user2.ID, "accepted")

	msgID, chatID, timestamp, err := saveChatMsg(db, user1.ID, user2.ID, "Hello")
	if err != nil {
		t.Fatalf("Failed to save chat message: %v", err)
	}
	if msgID <= 0 {
		t.Error("Expected positive message ID")
	}
	if chatID <= 0 {
		t.Error("Expected positive chat ID")
	}
	if timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	// Test no connection
	user3 := createTestUser(t, "savechat3@example.com", "password123")
	createTestProfile(t, user3, testProfile)
	_, _, _, err = saveChatMsg(db, user1.ID, user3.ID, "Should fail")
	if err == nil {
		t.Error("Expected error for users without connection")
	}
	if !strings.Contains(err.Error(), "no accepted connection") {
		t.Errorf("Expected no accepted connection error, got: %v", err)
	}
}

// Test getChatMessages function
func TestGetChatMessages(t *testing.T) {
	user1 := createTestUser(t, "getchat1@example.com", "password123")
	user2 := createTestUser(t, "getchat2@example.com", "password123")
	testProfile := getDefaultTestProfile()
	createTestProfile(t, user1, testProfile)
	createTestProfile(t, user2, testProfile)
	createConnection(t, user1.ID, user2.ID, "accepted")

	// Save test messages
	msg1ID, _, _, err := saveChatMsg(db, user1.ID, user2.ID, "First message")
	if err != nil {
		t.Fatalf("Failed to save first message: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	msg2ID, _, _, err := saveChatMsg(db, user2.ID, user1.ID, "Second message")
	if err != nil {
		t.Fatalf("Failed to save second message: %v", err)
	}

	messages, err := getChatMessages(db, user1.ID, user2.ID, 10, nil)
	if err != nil {
		t.Fatalf("Failed to get chat messages: %v", err)
	}
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
	// Should be in reverse chronological order (newest first)
	if messages[0].ID != msg2ID {
		t.Errorf("Expected first message ID %d, got %d", msg2ID, messages[0].ID)
	}
	if messages[1].ID != msg1ID {
		t.Errorf("Expected second message ID %d, got %d", msg1ID, messages[1].ID)
	}
}

// Test Hub functionality
func TestHubBasic(t *testing.T) {
	hub := newHub()
	client := &Client{
		userID: 456,
		send:   make(chan ServerEvent, 256),
	}
	hub.register(client)

	event := ServerEvent{Type: "test", Data: "hello"}
	hub.sendToUser(456, event)

	select {
	case received := <-client.send:
		if received.Type != "test" {
			t.Errorf("Expected type test, got %s", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Message was not received in time")
	}

	// Test unregister method
	t.Run("Hub unregister", func(t *testing.T) {
		// Verify client is registered
		if len(hub.clientsByUser[456]) != 1 {
			t.Errorf("Expected 1 client, got %d", len(hub.clientsByUser[456]))
		}

		// Unregister the client
		hub.unregister(client)

		// Verify client is unregistered
		if len(hub.clientsByUser[456]) != 0 {
			t.Errorf("Expected 0 clients after unregister, got %d", len(hub.clientsByUser[456]))
		}
	})
}

// Test getChatHistoryHandler
func TestGetChatHistoryHandler(t *testing.T) {
	// Create test users and data
	user1 := createTestUser(t, "histhand1@example.com", "password123")
	user2 := createTestUser(t, "histhand2@example.com", "password123")
	testProfile := getDefaultTestProfile()
	createTestProfile(t, user1, testProfile)
	createTestProfile(t, user2, testProfile)
	createConnection(t, user1.ID, user2.ID, "accepted")

	// Save test messages
	_, _, _, err := saveChatMsg(db, user1.ID, user2.ID, "Test message 1")
	if err != nil {
		t.Fatalf("Failed to save test message 1: %v", err)
	}
	_, _, _, err = saveChatMsg(db, user2.ID, user1.ID, "Test message 2")
	if err != nil {
		t.Fatalf("Failed to save test message 2: %v", err)
	}

	handler := getChatHistoryHandler(db)

	t.Run("Successful fetch", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/chats/%d/messages", user2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type application/json")
		}

		var messages []ChatMessage
		err := json.NewDecoder(w.Body).Decode(&messages)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if len(messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(messages))
		}
	})

	t.Run("With limit parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/chats/%d/messages?limit=1", user2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var messages []ChatMessage
		err := json.NewDecoder(w.Body).Decode(&messages)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if len(messages) != 1 {
			t.Errorf("Expected 1 message, got %d", len(messages))
		}
	})

	t.Run("Unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/chats/%d/messages", user2.ID), nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/chats/invalid/messages", nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Malformed path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/chats/123", nil) // missing /messages
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

// Test chatSummaryHandler
func TestChatSummaryHandler(t *testing.T) {
	// Create test users
	user1 := createTestUser(t, "chatsummary1@example.com", "password123")
	user2 := createTestUser(t, "chatsummary2@example.com", "password123")
	user3 := createTestUser(t, "chatsummary3@example.com", "password123")

	testProfile := getDefaultTestProfile()
	createTestProfile(t, user1, testProfile)
	createTestProfile(t, user2, testProfile)
	createTestProfile(t, user3, testProfile)

	// Create connections
	createConnection(t, user1.ID, user2.ID, "accepted")
	createConnection(t, user1.ID, user3.ID, "accepted")

	// Send some messages to create chat history
	_, _, _, err := saveChatMsg(db, user2.ID, user1.ID, "Hello from user2")
	if err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}
	_, _, _, err = saveChatMsg(db, user3.ID, user1.ID, "Hello from user3")
	if err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}

	handler := chatSummaryHandler(db)

	t.Run("Successful fetch", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/chat/summary", nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type application/json")
		}

		var summaries []ChatPeerSummary
		err := json.NewDecoder(w.Body).Decode(&summaries)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if len(summaries) != 2 {
			t.Errorf("Expected 2 summaries, got %d", len(summaries))
		}

		// Check that summaries contain expected data
		foundUser2, foundUser3 := false, false
		for _, summary := range summaries {
			if summary.UserID == user2.ID {
				foundUser2 = true
				if summary.UnreadMessages != 1 {
					t.Errorf("Expected 1 unread message from user2, got %d", summary.UnreadMessages)
				}
			}
			if summary.UserID == user3.ID {
				foundUser3 = true
				if summary.UnreadMessages != 1 {
					t.Errorf("Expected 1 unread message from user3, got %d", summary.UnreadMessages)
				}
			}
		}
		if !foundUser2 {
			t.Error("Expected user2 in summaries")
		}
		if !foundUser3 {
			t.Error("Expected user3 in summaries")
		}
	})

	t.Run("Unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/chat/summary", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

// Test chatsMarkReadHandler
func TestChatsMarkReadHandler(t *testing.T) {
	// Create test users and messages
	user1 := createTestUser(t, "markread1@example.com", "password123")
	user2 := createTestUser(t, "markread2@example.com", "password123")

	testProfile := getDefaultTestProfile()
	createTestProfile(t, user1, testProfile)
	createTestProfile(t, user2, testProfile)
	createConnection(t, user1.ID, user2.ID, "accepted")

	// Send message from user2 to user1 (creates unread for user1)
	_, chatID, _, err := saveChatMsg(db, user2.ID, user1.ID, "Unread message")
	if err != nil {
		t.Fatalf("Failed to save message: %v", err)
	}

	handler := chatsMarkReadHandler(db)

	t.Run("Successful mark as read", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/chats/read?peer_id=%d", user2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", w.Code)
		}

		// Verify message was marked as read
		var isRead bool
		err = db.QueryRow(`
			SELECT is_read FROM messages 
			WHERE chat_id = $1 AND sender_id = $2
		`, chatID, user2.ID).Scan(&isRead)
		if err != nil {
			t.Fatalf("Failed to check message read status: %v", err)
		}
		if !isRead {
			t.Error("Expected message to be marked as read")
		}
	})

	t.Run("Wrong HTTP method", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/chats/read?peer_id=%d", user2.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("Unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("POST", fmt.Sprintf("/chats/read?peer_id=%d", user2.ID), nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Invalid peer_id", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/chats/read?peer_id=invalid", nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Non-existent chat", func(t *testing.T) {
		user4 := createTestUser(t, "markread4@example.com", "password123")
		createTestProfile(t, user4, testProfile)

		req := httptest.NewRequest("POST", fmt.Sprintf("/chats/read?peer_id=%d", user4.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user1.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status 204 for non-existent chat, got %d", w.Code)
		}
	})
}
