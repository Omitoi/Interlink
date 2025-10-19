package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// Initialize JWT secret for helper tests
func init() {
	jwtSecret = []byte("test-secret-key-for-testing")
}

// createTestUser creates a user with the given email and password, returns TestUser with ID and Token
func createTestUser(t *testing.T, email, password string) TestUser {
	t.Helper()

	// Clean up existing user
	db.Exec("DELETE FROM users WHERE email = $1", email)

	// Create user
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}

	_, err = db.Exec("INSERT INTO users (email, password_hash) VALUES ($1, $2)", email, string(hash))
	if err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	// Get user ID
	var userID int
	err = db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		t.Fatalf("failed to get user ID: %v", err)
	}

	// Login to get token
	token := loginUser(t, email, password)

	return TestUser{
		ID:       userID,
		Email:    email,
		Password: password,
		Token:    token,
	}
}

// loginUser logs in a user and returns the JWT token
func loginUser(t *testing.T, email, password string) string {
	t.Helper()

	reqBody := []byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password))
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	loginHandler(db).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed for %s: status %d", email, w.Code)
	}

	var respBody map[string]string
	json.NewDecoder(w.Body).Decode(&respBody)
	token, ok := respBody["token"]
	if !ok {
		t.Fatalf("expected token in login response, got %v", respBody)
	}

	return token
}

// createTestProfile creates a complete profile for a user
func createTestProfile(t *testing.T, user TestUser, profile TestProfile) {
	t.Helper()

	// Clean up existing profile
	db.Exec("DELETE FROM profiles WHERE user_id = $1", user.ID)

	// Create profile via handler
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("failed to marshal profile: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/me/profile", bytes.NewBuffer(profileJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+user.Token)
	w := httptest.NewRecorder()

	completeProfileHandler(db).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("failed to create profile for user %d: status %d", user.ID, w.Code)
	}
}

// createConnection creates a connection between two users
func createConnection(t *testing.T, fromUserID, toUserID int, status string) {
	t.Helper()

	_, err := db.Exec("INSERT INTO connections (user_id, target_user_id, status) VALUES ($1, $2, $3)",
		fromUserID, toUserID, status)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
}

// getDefaultTestProfile returns a default profile for testing
func getDefaultTestProfile() TestProfile {
	return TestProfile{
		DisplayName:        "Test User",
		AboutMe:            "I love testing!",
		ProfilePictureFile: "",
		LocationCity:       "Testville",
		LocationLat:        12.34,
		LocationLon:        56.78,
		MaxRadiusKm:        100,
		AnalogPassions:     []string{"calligraphy"},
		DigitalDelights:    []string{"retro gaming"},
		CrossPollination:   "Looking for a D&D group",
		FavoriteFood:       "Pizza",
		FavoriteMusic:      "Jazz",
		OtherBio:           map[string]interface{}{"quirk": "Loves robots"},
		MatchPreferences: map[string]int{
			"analog_passions":         5,
			"digital_delights":        3,
			"collaboration_interests": 4,
			"favorite_food":           2,
			"favorite_music":          1,
			"location":                5,
		},
	}
}

// cleanupTestData removes test data for given emails
func cleanupTestData(emails ...string) {
	for _, email := range emails {
		db.Exec("DELETE FROM connections WHERE user_id IN (SELECT id FROM users WHERE email = $1) OR target_user_id IN (SELECT id FROM users WHERE email = $1)", email)
		db.Exec("DELETE FROM profiles WHERE user_id IN (SELECT id FROM users WHERE email = $1)", email)
		db.Exec("DELETE FROM users WHERE email = $1", email)
	}
}
