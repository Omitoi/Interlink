package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestGetUserAvatarHandler(t *testing.T) {
	// Create test users
	owner := createTestUser(t, "avatar_owner@example.com", "password123")
	friend := createTestUser(t, "avatar_friend@example.com", "password123")
	stranger := createTestUser(t, "avatar_stranger@example.com", "password123")
	recommended := createTestUser(t, "avatar_recommended@example.com", "password123")

	// Create profiles for all users
	testProfile := getDefaultTestProfile()
	createTestProfile(t, owner, testProfile)
	createTestProfile(t, friend, testProfile)
	createTestProfile(t, stranger, testProfile)
	createTestProfile(t, recommended, testProfile)

	// Create connection between owner and friend
	createConnection(t, owner.ID, friend.ID, "accepted")

	// Make sure recommended user is actually recommendable
	// First, ensure stranger won't be recommended by creating a dismissed relationship
	createConnection(t, owner.ID, stranger.ID, "dismissed")

	handler := getUserAvatarHandler(db)

	t.Run("Own avatar access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", owner.ID), nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "image/png" {
			t.Errorf("Expected Content-Type image/png, got %s", contentType)
		}

		cacheControl := w.Header().Get("Cache-Control")
		if cacheControl != "private, max-age=3600" {
			t.Errorf("Expected Cache-Control private, max-age=3600, got %s", cacheControl)
		}
	})

	t.Run("Connected user avatar access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", friend.ID), nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "image/png" {
			t.Errorf("Expected Content-Type image/png, got %s", contentType)
		}
	})

	t.Run("Stranger avatar access denied", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", stranger.ID), nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Wrong HTTP method", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/avatars/"+fmt.Sprintf("%d", owner.ID), nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("Invalid path format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/", nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/invalid", nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Non-existent user ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/99999", nil)
		req.Header.Set("Authorization", "Bearer "+owner.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", owner.ID), nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func TestGetUserAvatarHandlerWithCustomAvatar(t *testing.T) {
	// Create test user
	user := createTestUser(t, "custom_avatar@example.com", "password123")
	testProfile := getDefaultTestProfile()
	createTestProfile(t, user, testProfile)

	// Create a temporary custom avatar file
	customFilename := fmt.Sprintf("test_avatar_%d.jpg", user.ID)
	customPath := filepath.Join(avatarRoot, customFilename)

	// Create the custom file
	err := os.WriteFile(customPath, []byte("fake jpeg content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test avatar file: %v", err)
	}
	defer os.Remove(customPath) // Clean up

	// Update database to reference the custom file
	_, err = db.Exec("UPDATE profiles SET profile_picture_file = $1 WHERE user_id = $2", customFilename, user.ID)
	if err != nil {
		t.Fatalf("Failed to update profile picture: %v", err)
	}

	handler := getUserAvatarHandler(db)

	t.Run("Custom JPEG avatar", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", user.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "image/jpeg" {
			t.Errorf("Expected Content-Type image/jpeg, got %s", contentType)
		}
	})

	// Test with PNG extension
	pngFilename := fmt.Sprintf("test_avatar_%d.png", user.ID)
	pngPath := filepath.Join(avatarRoot, pngFilename)

	err = os.WriteFile(pngPath, []byte("fake png content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test PNG avatar file: %v", err)
	}
	defer os.Remove(pngPath)

	_, err = db.Exec("UPDATE profiles SET profile_picture_file = $1 WHERE user_id = $2", pngFilename, user.ID)
	if err != nil {
		t.Fatalf("Failed to update profile picture to PNG: %v", err)
	}

	t.Run("Custom PNG avatar", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", user.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "image/png" {
			t.Errorf("Expected Content-Type image/png, got %s", contentType)
		}
	})
}

func TestGetUserAvatarHandlerMissingCustomFile(t *testing.T) {
	// Create test user
	user := createTestUser(t, "missing_avatar@example.com", "password123")
	testProfile := getDefaultTestProfile()
	createTestProfile(t, user, testProfile)

	// Set a custom filename in database but don't create the actual file
	missingFilename := "non_existent_file.jpg"
	_, err := db.Exec("UPDATE profiles SET profile_picture_file = $1 WHERE user_id = $2", missingFilename, user.ID)
	if err != nil {
		t.Fatalf("Failed to update profile picture: %v", err)
	}

	handler := getUserAvatarHandler(db)

	t.Run("Missing custom file falls back to placeholder", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/avatars/"+fmt.Sprintf("%d", user.ID), nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if contentType != "image/png" {
			t.Errorf("Expected Content-Type image/png (placeholder), got %s", contentType)
		}
	})
}
