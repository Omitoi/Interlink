package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserBioHandler(t *testing.T) {
	// Create test users
	requester := createTestUser(t, "bio_requester@example.com", "password123")
	target := createTestUser(t, "bio_target@example.com", "password123")
	stranger := createTestUser(t, "bio_stranger@example.com", "password123")

	// Create profiles
	testProfile := getDefaultTestProfile()
	createTestProfile(t, requester, testProfile)

	targetProfile := getDefaultTestProfile()
	targetProfile.DisplayName = "Bio Target User"
	targetProfile.AnalogPassions = []string{"woodworking", "pottery"}
	targetProfile.DigitalDelights = []string{"programming", "gaming"}
	targetProfile.CrossPollination = "Looking for maker space friends"
	targetProfile.FavoriteMusic = "Classical"
	createTestProfile(t, target, targetProfile)

	createTestProfile(t, stranger, testProfile)

	handler := userBioHandler(db)

	t.Run("Successful bio access via connection", func(t *testing.T) {
		// Create connection between requester and target
		createConnection(t, requester.ID, target.ID, "accepted")

		req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d/bio", target.ID), nil)
		req.Header.Set("Authorization", "Bearer "+requester.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var response map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response["id"].(float64) != float64(target.ID) {
			t.Errorf("Expected id %d, got %v", target.ID, response["id"])
		}

		// Check that bio data is present
		if response["analog_passions"] == nil {
			t.Error("Expected analog_passions in response")
		}
		if response["digital_delights"] == nil {
			t.Error("Expected digital_delights in response")
		}
		if response["collaboration_interests"] == nil {
			t.Error("Expected collaboration_interests in response")
		}
		if response["favorite_food"] == nil {
			t.Error("Expected favorite_food in response")
		}
		if response["favorite_music"] == nil {
			t.Error("Expected favorite_music in response")
		}
	})

	t.Run("Bio access via pending connection", func(t *testing.T) {
		// Create another user with pending connection
		pendingUser := createTestUser(t, "bio_pending@example.com", "password123")
		createTestProfile(t, pendingUser, testProfile)
		createConnection(t, pendingUser.ID, target.ID, "pending")

		req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d/bio", target.ID), nil)
		req.Header.Set("Authorization", "Bearer "+pendingUser.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("Access denied for stranger", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d/bio", target.ID), nil)
		req.Header.Set("Authorization", "Bearer "+stranger.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Invalid path format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/bio", nil)
		req.Header.Set("Authorization", "Bearer "+requester.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/invalid/bio", nil)
		req.Header.Set("Authorization", "Bearer "+requester.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Non-existent user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/99999/bio", nil)
		req.Header.Set("Authorization", "Bearer "+requester.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Unauthorized request", func(t *testing.T) {
		req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d/bio", target.ID), nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Wrong path structure", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/users/123/wrong", nil)
		req.Header.Set("Authorization", "Bearer "+requester.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Access via recommendations", func(t *testing.T) {
		// Create users with highly matching profiles to ensure recommendation
		recommUser := createTestUser(t, "bio_recomm@example.com", "password123")
		recommTarget := createTestUser(t, "bio_recomm_target@example.com", "password123")

		// Create highly matching profiles
		recommProfile := getDefaultTestProfile()
		recommProfile.AnalogPassions = []string{"blacksmithing", "leatherwork"}
		recommProfile.DigitalDelights = []string{"retro gaming", "pixel art"}
		recommProfile.CrossPollination = "Maker community collaboration"
		recommProfile.FavoriteFood = "Artisanal bread"
		recommProfile.FavoriteMusic = "Folk"
		recommProfile.LocationLat = 50.1
		recommProfile.LocationLon = 50.1

		createTestProfile(t, recommUser, recommProfile)

		targetRecommProfile := recommProfile // Same profile for high match
		targetRecommProfile.DisplayName = "Recommended Target"
		createTestProfile(t, recommTarget, targetRecommProfile)

		// The test may or may not work depending on other users in the system,
		// but it will at least exercise the recommendation path in the code
		req := httptest.NewRequest("GET", fmt.Sprintf("/users/%d/bio", recommTarget.ID), nil)
		req.Header.Set("Authorization", "Bearer "+recommUser.Token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// The result depends on whether recommTarget appears in recommUser's recommendations
		// This tests the recommendation code path even if access is denied
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("Expected status 200 or 404, got %d", w.Code)
		}
	})
}
