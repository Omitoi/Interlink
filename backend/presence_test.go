package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMePingHandler(t *testing.T) {
	// Create test user
	user := createTestUser(t, "ping_test@example.com", "password123")

	handler := mePingHandler(db)

	t.Run("Successful ping", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/me/ping", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", w.Code)
		}

		// Verify last_online was updated - check that user is now online
		online, err := isOnlineNow(db, user.ID)
		if err != nil {
			t.Fatalf("Failed to check if user is online: %v", err)
		}
		if !online {
			t.Error("Expected user to be online after ping")
		}
	})

	t.Run("Wrong HTTP method", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/me/ping", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("Unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/me/ping", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Invalid token", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/me/ping", nil)
		req.Header.Set("Authorization", "Bearer invalid_token")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func TestIsOnlineNow(t *testing.T) {
	// Create test user
	user := createTestUser(t, "online_test@example.com", "password123")

	t.Run("User is online after recent ping", func(t *testing.T) {
		// Update user's last_online to now
		_, err := db.Exec("UPDATE users SET last_online = NOW() WHERE id = $1", user.ID)
		if err != nil {
			t.Fatalf("Failed to update last_online: %v", err)
		}

		online, err := isOnlineNow(db, user.ID)
		if err != nil {
			t.Fatalf("isOnlineNow failed: %v", err)
		}

		if !online {
			t.Error("Expected user to be online")
		}
	})

	t.Run("User is offline after old timestamp", func(t *testing.T) {
		// Update user's last_online to 2 minutes ago (beyond 90 second threshold)
		_, err := db.Exec("UPDATE users SET last_online = NOW() - INTERVAL '2 minutes' WHERE id = $1", user.ID)
		if err != nil {
			t.Fatalf("Failed to update last_online: %v", err)
		}

		online, err := isOnlineNow(db, user.ID)
		if err != nil {
			t.Fatalf("isOnlineNow failed: %v", err)
		}

		if online {
			t.Error("Expected user to be offline")
		}
	})

	t.Run("Non-existent user", func(t *testing.T) {
		online, err := isOnlineNow(db, 99999)
		if err != nil {
			t.Fatalf("isOnlineNow should not error for non-existent user: %v", err)
		}

		if online {
			t.Error("Expected non-existent user to be offline")
		}
	})

	t.Run("User with NULL last_online", func(t *testing.T) {
		// Create user with NULL last_online
		userNull := createTestUser(t, "null_online@example.com", "password123")
		_, err := db.Exec("UPDATE users SET last_online = NULL WHERE id = $1", userNull.ID)
		if err != nil {
			t.Fatalf("Failed to set last_online to NULL: %v", err)
		}

		online, err := isOnlineNow(db, userNull.ID)
		if err != nil {
			t.Fatalf("isOnlineNow failed for NULL last_online: %v", err)
		}

		if online {
			t.Error("Expected user with NULL last_online to be offline")
		}
	})
}

// Test presence integration with mePingHandler
func TestPresenceIntegration(t *testing.T) {
	user1 := createTestUser(t, "presence1@example.com", "password123")
	user2 := createTestUser(t, "presence2@example.com", "password123")

	// Ensure both users start with NULL last_online
	_, err := db.Exec("UPDATE users SET last_online = NULL WHERE id IN ($1, $2)", user1.ID, user2.ID)
	if err != nil {
		t.Fatalf("Failed to reset last_online: %v", err)
	}

	handler := mePingHandler(db)

	// Both users start offline
	online1, _ := isOnlineNow(db, user1.ID)
	online2, _ := isOnlineNow(db, user2.ID)
	if online1 || online2 {
		t.Error("Expected both users to start offline")
	}

	// User1 pings
	req := httptest.NewRequest("POST", "/me/ping", nil)
	req.Header.Set("Authorization", "Bearer "+user1.Token)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Now user1 should be online, user2 still offline
	online1, _ = isOnlineNow(db, user1.ID)
	online2, _ = isOnlineNow(db, user2.ID)
	if !online1 {
		t.Error("Expected user1 to be online after ping")
	}
	if online2 {
		t.Error("Expected user2 to still be offline")
	}

	// User2 pings
	req = httptest.NewRequest("POST", "/me/ping", nil)
	req.Header.Set("Authorization", "Bearer "+user2.Token)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Now both should be online
	online1, _ = isOnlineNow(db, user1.ID)
	online2, _ = isOnlineNow(db, user2.ID)
	if !online1 || !online2 {
		t.Error("Expected both users to be online after pings")
	}
}
