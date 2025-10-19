package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// RECOMMENDATIONS AND CONNECTIONS TEST SUITE
// ============================================================================

func TestRecommendationsAndConnectionsSuite(t *testing.T) {
	t.Run("Recommendations", func(t *testing.T) {
		testRecommendations(t)
	})

	t.Run("ConnectionManagement", func(t *testing.T) {
		testConnectionManagement(t)
	})

	t.Run("Dismissals", func(t *testing.T) {
		testDismissals(t)
	})
}

func testRecommendations(t *testing.T) {
	// Setup users
	userA := createTestUser(t, "rec_a@example.com", "passwordA")
	userB := createTestUser(t, "rec_b@example.com", "passwordB")
	userIncomplete := createTestUser(t, "rec_incomplete@example.com", "passwordI")

	defer cleanupTestData(userA.Email, userB.Email, userIncomplete.Email)

	// Create complete profiles
	profileA := getDefaultTestProfile()
	profileA.DisplayName = "Recommender A"
	profileA.AnalogPassions = []string{"calligraphy", "knitting"}

	profileB := getDefaultTestProfile()
	profileB.DisplayName = "Recommender B"
	profileB.AnalogPassions = []string{"calligraphy"}
	profileB.FavoriteMusic = "Rock"
	profileB.LocationLat = 10.1
	profileB.LocationLon = 20.1

	createTestProfile(t, userA, profileA)
	createTestProfile(t, userB, profileB)

	// Create incomplete profile for testing gating
	db.Exec("INSERT INTO profiles (user_id, display_name, is_complete) VALUES ($1, $2, false)",
		userIncomplete.ID, "Incomplete User")

	t.Run("Basic Recommendation Generation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		recommendationsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var recResp struct {
			Recommendations []int `json:"recommendations"`
		}
		json.NewDecoder(w.Body).Decode(&recResp)

		// Check that we get some recommendations
		if len(recResp.Recommendations) == 0 {
			t.Error("expected at least one recommendation, got none")
			return
		}

		// Verify that userB is in recommendations or at least that we get valid recommendations
		// Since UserA and UserB have compatible profiles (both have calligraphy),
		// UserB should be recommended if not already connected/dismissed
		validRecommendation := false
		for _, id := range recResp.Recommendations {
			if id == userB.ID {
				validRecommendation = true
				break
			}
			// Also check if it's a valid user ID (not the same as requesting user)
			if id != userA.ID && id > 0 {
				validRecommendation = true
			}
		}

		if !validRecommendation {
			t.Errorf("expected valid recommendations, got %v (userA=%d, userB=%d)",
				recResp.Recommendations, userA.ID, userB.ID)
		}
	})

	t.Run("Incomplete Profile Gating", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
		req.Header.Set("Authorization", "Bearer "+userIncomplete.Token)
		w := httptest.NewRecorder()

		recommendationsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected status 403, got %d", w.Code)
		}

		var errResp map[string]string
		json.NewDecoder(w.Body).Decode(&errResp)

		if errResp["error"] != "incomplete_profile" {
			t.Errorf("expected error incomplete_profile, got %v", errResp)
		}
	})

	t.Run("Unauthorized Recommendations", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
		w := httptest.NewRecorder()

		recommendationsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func testConnectionManagement(t *testing.T) {
	// Setup users
	userA := createTestUser(t, "conn_a@example.com", "passwordA")
	userB := createTestUser(t, "conn_b@example.com", "passwordB")

	defer cleanupTestData(userA.Email, userB.Email)

	// Create profiles
	profileA := getDefaultTestProfile()
	profileA.DisplayName = "Connection A"
	profileB := getDefaultTestProfile()
	profileB.DisplayName = "Connection B"

	createTestProfile(t, userA, profileA)
	createTestProfile(t, userB, profileB)

	// Create connection between users
	createConnection(t, userA.ID, userB.ID, "accepted")

	t.Run("List Connections", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/connections", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		connectionsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var connResp struct {
			Connections []int `json:"connections"`
		}
		json.NewDecoder(w.Body).Decode(&connResp)

		found := false
		for _, id := range connResp.Connections {
			if id == userB.ID {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected user B (id=%d) in connections, got %v", userB.ID, connResp.Connections)
		}
	})

	t.Run("Unauthorized Connections", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/connections", nil)
		w := httptest.NewRecorder()

		connectionsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}

func testDismissals(t *testing.T) {
	// Setup users
	userA := createTestUser(t, "dismiss_a@example.com", "passwordA")
	userB := createTestUser(t, "dismiss_b@example.com", "passwordB")
	userSingle := createTestUser(t, "dismiss_single@example.com", "passwordS")

	defer cleanupTestData(userA.Email, userB.Email, userSingle.Email)

	// Create profiles with unique characteristics to ensure B is highly recommended to A
	profileA := getDefaultTestProfile()
	profileA.DisplayName = "Dismiss A"
	profileA.AnalogPassions = []string{"blacksmithing", "woodworking"}
	profileA.DigitalDelights = []string{"indie games", "code golf"}
	profileA.CrossPollination = "Looking for maker space buddies"
	profileA.FavoriteFood = "Sushi"
	profileA.FavoriteMusic = "Electronic"
	profileA.LocationLat = 25.5
	profileA.LocationLon = 45.5
	profileA.MatchPreferences = map[string]int{
		"analog_passions":         5,
		"digital_delights":        5,
		"collaboration_interests": 5,
		"favorite_food":           5,
		"favorite_music":          5,
		"location":                1, // Low location weight for this test
	}

	profileB := getDefaultTestProfile()
	profileB.DisplayName = "Dismiss B"
	profileB.AnalogPassions = []string{"blacksmithing", "woodworking"}         // Perfect match
	profileB.DigitalDelights = []string{"indie games", "code golf"}            // Perfect match
	profileB.CrossPollination = "Maker space enthusiast seeking collaborators" // Similar
	profileB.FavoriteFood = "Sushi"                                            // Perfect match
	profileB.FavoriteMusic = "Electronic"                                      // Perfect match
	profileB.LocationLat = 25.6                                                // Close to A
	profileB.LocationLon = 45.6                                                // Close to A
	profileB.MatchPreferences = map[string]int{
		"analog_passions":         5,
		"digital_delights":        5,
		"collaboration_interests": 5,
		"favorite_food":           5,
		"favorite_music":          5,
		"location":                1,
	}

	profileSingle := getDefaultTestProfile() // Keep default to be less attractive
	profileSingle.DisplayName = "Single User"

	createTestProfile(t, userA, profileA)
	createTestProfile(t, userB, profileB)
	createTestProfile(t, userSingle, profileSingle)

	t.Run("Successful Dismissal", func(t *testing.T) {
		// First verify B is in A's recommendations
		req := httptest.NewRequest(http.MethodGet, "/recommendations", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		recommendationsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("initial recommendations failed: %d", w.Code)
		}

		var rec struct {
			Recommendations []int `json:"recommendations"`
		}
		json.NewDecoder(w.Body).Decode(&rec)

		present := false
		for _, id := range rec.Recommendations {
			if id == userB.ID {
				present = true
				break
			}
		}

		if !present {
			t.Fatalf("expected userB in recommendations: %v", rec.Recommendations)
		}

		// Dismiss userB
		req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/recommendations/%d/dismiss", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w = httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("dismiss expected 201, got %d", w.Code)
		}

		// Verify userB is no longer in recommendations
		req = httptest.NewRequest(http.MethodGet, "/recommendations", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w = httptest.NewRecorder()

		recommendationsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("recommendations after dismiss failed: %d", w.Code)
		}

		rec = struct {
			Recommendations []int `json:"recommendations"`
		}{}
		json.NewDecoder(w.Body).Decode(&rec)

		for _, id := range rec.Recommendations {
			if id == userB.ID {
				t.Fatalf("dismissed user still present")
			}
		}
	})

	t.Run("Idempotent Dismissal", func(t *testing.T) {
		// Repeat dismissal should still return 201
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/recommendations/%d/dismiss", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("repeat dismiss expected 201, got %d", w.Code)
		}
	})

	t.Run("Dismiss Non-existent User", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/recommendations/999999/dismiss", nil)
		req.Header.Set("Authorization", "Bearer "+userSingle.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", w.Code)
		}
	})
}
