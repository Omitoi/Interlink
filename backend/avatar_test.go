package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// AVATAR SYSTEM TEST SUITE
// ============================================================================

// TestUser structure for avatar tests
type AvatarTestUser struct {
	ID    int
	Email string
	Token string
}

func TestAvatarSystemSuite(t *testing.T) {
	t.Run("MyAvatarHandler", func(t *testing.T) {
		testMyAvatarHandler(t)
	})

	t.Run("HelperFunctions", func(t *testing.T) {
		testAvatarHelperFunctions(t)
	})
}

// Helper function to create test users with complete profiles for avatar testing
func createTestUserForAvatars(t *testing.T, email, password string) AvatarTestUser {
	// Clean up any existing user first
	db.Exec("DELETE FROM users WHERE email = $1", email)

	// Create user
	regPayload := map[string]string{
		"email":    email,
		"password": password,
	}
	regBody, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(regBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerHandler(db).ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create test user %s: %d", email, w.Code)
	}

	// Login to get token
	loginPayload := map[string]string{
		"email":    email,
		"password": password,
	}
	loginBody, _ := json.Marshal(loginPayload)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(loginBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	loginHandler(db).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Failed to login test user %s: %d", email, w.Code)
	}

	var loginResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &loginResp)

	userID := int(loginResp["id"].(float64))
	token := loginResp["token"].(string)

	// Complete profile to enable avatar functionality
	profilePayload := map[string]interface{}{
		"display_name":            fmt.Sprintf("Test User %d", userID),
		"about_me":                "Test bio for avatar testing",
		"location_city":           "Test City",
		"location_lat":            60.1699,
		"location_lon":            24.9384,
		"max_radius_km":           50,
		"analog_passions":         []string{"testing", "avatars"},
		"digital_delights":        []string{"coding", "images"},
		"collaboration_interests": "Looking for avatar connections",
		"favorite_food":           "pizza",
		"favorite_music":          "electronic",
		"other_bio":               map[string]interface{}{},
		"match_preferences": map[string]interface{}{
			"analog_passions":         2,
			"digital_delights":        2,
			"collaboration_interests": 2,
			"favorite_food":           1,
			"favorite_music":          1,
			"location":                5,
		},
	}
	profileBody, _ := json.Marshal(profilePayload)
	req = httptest.NewRequest(http.MethodPost, "/complete-profile", bytes.NewBuffer(profileBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	completeProfileHandler(db).ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Logf("Profile completion response: %d - %s", w.Code, w.Body.String())
		// Don't fail the test if profile completion has issues - the user still exists
	}

	return AvatarTestUser{
		ID:    userID,
		Email: email,
		Token: token,
	}
}

func cleanupAvatarTestData(userEmails ...string) {
	for _, email := range userEmails {
		var userID int
		db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)

		// Clean up avatar files
		filename, err := getProfilePictureFilename(db, userID)
		if err == nil && filename != "" {
			avatarPath := filepath.Join("./uploads/avatars", filename)
			os.Remove(avatarPath)
		}

		// Clean up database entries
		db.Exec("DELETE FROM connections WHERE user_id = $1 OR target_user_id = $1", userID)
		db.Exec("DELETE FROM profiles WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}
}

// Helper function to create a test JPEG file content
func createTestJPEGContent() []byte {
	// Minimal valid JPEG file header
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xFF, 0xDB, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0A, 0x0C, 0x14, 0x0D, 0x0C, 0x0B, 0x0B, 0x0C, 0x19, 0x12,
		0x13, 0x0F, 0x14, 0x1D, 0x1A, 0x1F, 0x1E, 0x1D, 0x1A, 0x1C, 0x1C, 0x20,
		0x24, 0x2E, 0x27, 0x20, 0x22, 0x2C, 0x23, 0x1C, 0x1C, 0x28, 0x37, 0x29,
		0x2C, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1F, 0x27, 0x39, 0x3D, 0x38, 0x32,
		0x3C, 0x2E, 0x33, 0x34, 0x32, 0xFF, 0xD9,
	}
}

// Helper function to create multipart form data for file upload
func createMultipartFormData(fieldName, fileName string, fileContent []byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile(fieldName, fileName)
	part.Write(fileContent)
	writer.Close()
	return body, writer.FormDataContentType()
}

// ============================================================================
// MY AVATAR HANDLER TESTS
// ============================================================================

func testMyAvatarHandler(t *testing.T) {
	user := createTestUserForAvatars(t, "avatar_test@example.com", "password123")
	defer cleanupAvatarTestData("avatar_test@example.com")

	// Ensure uploads directory exists
	os.MkdirAll("./uploads/avatars", 0o755)

	t.Run("Valid JPEG Upload", func(t *testing.T) {
		jpegContent := createTestJPEGContent()
		body, contentType := createMultipartFormData("file", "test.jpg", jpegContent)
		req := httptest.NewRequest(http.MethodPost, "/me/avatar", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		myAvatarHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
			if val, ok := resp["ok"]; !ok || val != true {
				t.Errorf("Expected 'ok': true in response, got: %v", resp)
			}
		} else {
			t.Errorf("Failed to parse JSON response: %v", err)
		}
	})

	t.Run("Invalid Method GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/me/avatar", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		myAvatarHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "method_not_allowed") {
			t.Errorf("Expected 'method_not_allowed' in response body")
		}
	})

	t.Run("Missing File", func(t *testing.T) {
		// Create request without file field
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/me/avatar", body)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		myAvatarHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "missing_file") {
			t.Errorf("Expected 'missing_file' in response body")
		}
	})

	t.Run("Delete Avatar", func(t *testing.T) {
		// First upload an avatar to delete
		jpegContent := createTestJPEGContent()
		body, contentType := createMultipartFormData("file", "test.jpg", jpegContent)
		req := httptest.NewRequest(http.MethodPost, "/me/avatar", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()
		myAvatarHandler(db).ServeHTTP(w, req)

		// Now delete it
		req = httptest.NewRequest(http.MethodDelete, "/me/avatar", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w = httptest.NewRecorder()

		myAvatarHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err == nil {
			if val, ok := resp["ok"]; !ok || val != true {
				t.Errorf("Expected 'ok': true in response, got: %v", resp)
			}
		}
	})

	t.Run("File Too Large", func(t *testing.T) {
		// Create a large file content (over 3MB limit)
		largeContent := make([]byte, 4<<20) // 4MB
		body, contentType := createMultipartFormData("file", "large.jpg", largeContent)
		req := httptest.NewRequest(http.MethodPost, "/me/avatar", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		myAvatarHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("Expected status 413, got %d", w.Code)
		}
	})

	t.Run("Invalid Content Type", func(t *testing.T) {
		// Create a PNG content (not JPEG)
		pngContent := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
		body, contentType := createMultipartFormData("file", "test.png", pngContent)
		req := httptest.NewRequest(http.MethodPost, "/me/avatar", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		myAvatarHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}

		if !strings.Contains(w.Body.String(), "only_jpeg_allowed") {
			t.Errorf("Expected 'only_jpeg_allowed' in response body")
		}
	})
}

// ============================================================================
// AVATAR HELPER FUNCTIONS TESTS
// ============================================================================

func testAvatarHelperFunctions(t *testing.T) {
	userA := createTestUserForAvatars(t, "helper_a@example.com", "password123")
	userB := createTestUserForAvatars(t, "helper_b@example.com", "password123")
	defer cleanupAvatarTestData("helper_a@example.com", "helper_b@example.com")

	t.Run("hasPendingOrAccepted", func(t *testing.T) {
		// Test no relationship
		exists, err := hasPendingOrAccepted(db, userA.ID, userB.ID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if exists {
			t.Errorf("Expected no relationship, but found one")
		}

		// Create pending connection
		db.Exec("INSERT INTO connections (user_id, target_user_id, status, created_at) VALUES ($1, $2, 'pending', NOW())",
			userA.ID, userB.ID)

		exists, err = hasPendingOrAccepted(db, userA.ID, userB.ID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !exists {
			t.Errorf("Expected pending relationship to be found")
		}

		// Update to accepted
		db.Exec("UPDATE connections SET status = 'accepted' WHERE user_id = $1 AND target_user_id = $2",
			userA.ID, userB.ID)

		exists, err = hasPendingOrAccepted(db, userA.ID, userB.ID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !exists {
			t.Errorf("Expected accepted relationship to be found")
		}

		// Update to dismissed (should not be found)
		db.Exec("UPDATE connections SET status = 'dismissed' WHERE user_id = $1 AND target_user_id = $2",
			userA.ID, userB.ID)

		exists, err = hasPendingOrAccepted(db, userA.ID, userB.ID)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if exists {
			t.Errorf("Expected dismissed relationship not to be found")
		}

		// Clean up
		db.Exec("DELETE FROM connections WHERE user_id = $1 AND target_user_id = $2", userA.ID, userB.ID)
	})

	t.Run("getProfilePictureFilename", func(t *testing.T) {
		// Test no filename set
		_, err := getProfilePictureFilename(db, userA.ID)
		if err == nil {
			t.Errorf("Expected error for no filename, but got none")
		}

		// Set filename
		testFilename := fmt.Sprintf("%d.jpg", userA.ID)
		db.Exec("UPDATE profiles SET profile_picture_file = $1 WHERE user_id = $2", testFilename, userA.ID)

		filename, err := getProfilePictureFilename(db, userA.ID)
		if err != nil {
			t.Errorf("Unexpected error getting filename: %v", err)
		}
		if filename != testFilename {
			t.Errorf("Expected filename %s, got %s", testFilename, filename)
		}

		// Test empty filename
		db.Exec("UPDATE profiles SET profile_picture_file = '' WHERE user_id = $1", userA.ID)
		_, err = getProfilePictureFilename(db, userA.ID)
		if err == nil {
			t.Errorf("Expected error for empty filename, but got none")
		}

		// Clean up
		db.Exec("UPDATE profiles SET profile_picture_file = NULL WHERE user_id = $1", userA.ID)
	})

	t.Run("removeAvatar", func(t *testing.T) {
		// Test removing non-existent avatar - should return error since no filename is set
		err := removeAvatar(db, userA.ID)
		if err == nil {
			t.Errorf("Expected error removing avatar when no filename set, but got none")
		}

		// Create test avatar file and database entry
		testFilename := fmt.Sprintf("%d.jpg", userA.ID)
		avatarPath := filepath.Join("./uploads/avatars", testFilename)
		os.MkdirAll("./uploads/avatars", 0o755)
		os.WriteFile(avatarPath, createTestJPEGContent(), 0o644)
		db.Exec("UPDATE profiles SET profile_picture_file = $1 WHERE user_id = $2", testFilename, userA.ID)

		// Verify file exists
		if _, err := os.Stat(avatarPath); os.IsNotExist(err) {
			t.Fatalf("Test avatar file was not created")
		}

		// Remove avatar
		err = removeAvatar(db, userA.ID)
		if err != nil {
			t.Errorf("Unexpected error removing avatar: %v", err)
		}

		// Verify file is removed
		if _, err := os.Stat(avatarPath); !os.IsNotExist(err) {
			t.Errorf("Avatar file was not removed")
		}

		// Verify database is cleared
		filename, err := getProfilePictureFilename(db, userA.ID)
		if err == nil {
			t.Errorf("Expected database entry to be cleared, but found filename: %s", filename)
		}
	})
}
