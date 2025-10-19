package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Initialize JWT secret for handler tests
func init() {
	jwtSecret = []byte("test-secret-key-for-testing")
}

// ============================================================================
// ADDITIONAL HANDLER TESTS FOR BETTER COVERAGE
// ============================================================================

func TestAdditionalHandlerCoverage(t *testing.T) {
	t.Run("UserHandler Edge Cases", func(t *testing.T) {
		testUserHandlerEdgeCases(t)
	})

	t.Run("DismissRecommendation Edge Cases", func(t *testing.T) {
		testDismissRecommendationEdgeCases(t)
	})

	t.Run("UserBioHandler Edge Cases", func(t *testing.T) {
		testUserBioHandlerEdgeCases(t)
	})

	t.Run("JsonRawOrArray Function", func(t *testing.T) {
		testJSONRawOrArray(t)
	})

	t.Run("Login Handler Edge Cases", func(t *testing.T) {
		testLoginHandlerEdgeCases(t)
	})

	t.Run("Register Handler Edge Cases", func(t *testing.T) {
		testRegisterHandlerEdgeCases(t)
	})
}

func testUserHandlerEdgeCases(t *testing.T) {
	// Setup test users
	userA := createTestUserForHandler(t, "user_handler_a@example.com", "passwordA")
	userB := createTestUserForHandler(t, "user_handler_b@example.com", "passwordB")

	defer cleanupTestDataForHandler(userA.Email, userB.Email)

	// Create profiles for both users
	profileA := getDefaultTestProfileForHandler()
	profileA.DisplayName = "User Handler A"
	profileB := getDefaultTestProfileForHandler()
	profileB.DisplayName = "User Handler B"

	createTestProfileForHandler(t, userA, profileA)
	createTestProfileForHandler(t, userB, profileB)

	t.Run("Valid User Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/users/%d", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		userHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)

		if resp["id"] != float64(userB.ID) {
			t.Errorf("Expected ID %d, got %v", userB.ID, resp["id"])
		}
	})

	t.Run("Invalid User ID Format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/invalid", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		userHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Non-existent User ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/999999", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		userHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func testDismissRecommendationEdgeCases(t *testing.T) {
	userA := createTestUserForHandler(t, "dismiss_a@example.com", "passwordA")
	userB := createTestUserForHandler(t, "dismiss_b@example.com", "passwordB")

	defer cleanupTestDataForHandler(userA.Email, userB.Email)

	// Create profiles
	profileA := getDefaultTestProfileForHandler()
	profileB := getDefaultTestProfileForHandler()
	createTestProfileForHandler(t, userA, profileA)
	createTestProfileForHandler(t, userB, profileB)

	t.Run("Wrong HTTP Method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/recommendations/%d/dismiss", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})

	t.Run("Invalid URL Format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/recommendations/invalid/dismiss", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Invalid User ID in URL", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/recommendations/abc/dismiss", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Dismiss Self", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/recommendations/%d/dismiss", userA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Valid Dismiss", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/recommendations/%d/dismiss", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		dismissRecommendationHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}
	})
}

func testUserBioHandlerEdgeCases(t *testing.T) {
	userA := createTestUserForHandler(t, "bio_a@example.com", "passwordA")
	userB := createTestUserForHandler(t, "bio_b@example.com", "passwordB")

	defer cleanupTestDataForHandler(userA.Email, userB.Email)

	// Create profiles
	profileA := getDefaultTestProfileForHandler()
	profileB := getDefaultTestProfileForHandler()
	createTestProfileForHandler(t, userA, profileA)
	createTestProfileForHandler(t, userB, profileB)

	t.Run("Invalid URL Format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/invalid/bio", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		userBioHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("Non-numeric User ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/users/abc/bio", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		userBioHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func testJSONRawOrArray(t *testing.T) {
	t.Run("Valid JSON Array", func(t *testing.T) {
		input := json.RawMessage(`["item1", "item2", "item3"]`)
		result := jsonRawOrArray(input)

		// Cast to slice
		if arr, ok := result.([]interface{}); ok {
			expected := []interface{}{"item1", "item2", "item3"}
			if len(arr) != len(expected) {
				t.Errorf("Expected length %d, got %d", len(expected), len(arr))
			}
		} else {
			t.Errorf("Expected array result, got %T", result)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		input := json.RawMessage(`invalid json`)
		result := jsonRawOrArray(input)

		// Should return empty array on invalid JSON
		if arr, ok := result.([]interface{}); ok {
			if len(arr) != 0 {
				t.Errorf("Expected empty array for invalid JSON, got %v", result)
			}
		} else {
			t.Errorf("Expected array result for invalid JSON, got %T", result)
		}
	})

	t.Run("Empty JSON", func(t *testing.T) {
		input := json.RawMessage(``)
		result := jsonRawOrArray(input)

		// Should return empty array
		if arr, ok := result.([]interface{}); ok {
			if len(arr) != 0 {
				t.Errorf("Expected empty array for empty JSON, got %v", result)
			}
		} else {
			t.Errorf("Expected array result for empty JSON, got %T", result)
		}
	})
}

func testLoginHandlerEdgeCases(t *testing.T) {
	// Create a test user first
	email := "login_edge_test@example.com"
	password := "testpassword123"

	// Clean up any existing user
	db.Exec("DELETE FROM users WHERE email = $1", email)
	defer db.Exec("DELETE FROM users WHERE email = $1", email)

	// Create user via registration
	registerData := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(registerData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	registerHandler(db).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create test user for login edge cases: %d", w.Code)
	}

	t.Run("Wrong Password", func(t *testing.T) {
		loginData := fmt.Sprintf(`{"email":"%s","password":"wrongpassword"}`, email)
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		loginHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("Non-existent Email", func(t *testing.T) {
		loginData := `{"email":"nonexistent@example.com","password":"anypassword"}`
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		loginHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})
}

func testRegisterHandlerEdgeCases(t *testing.T) {
	t.Run("Empty Email Field", func(t *testing.T) {
		registerData := `{"email":"","password":"testpassword"}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(registerData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		registerHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Empty Password Field", func(t *testing.T) {
		registerData := `{"email":"test@example.com","password":""}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(registerData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		registerHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("Whitespace Only Fields", func(t *testing.T) {
		registerData := `{"email":"   ","password":"   "}`
		req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(registerData))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		registerHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})
}

// Helper functions for this test file
func createTestUserForHandler(t *testing.T, email, password string) *TestUser {
	// Clean up any existing user first
	db.Exec("DELETE FROM users WHERE email = $1", email)

	registerData := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(registerData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	registerHandler(db).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Failed to create test user: %d", w.Code)
	}

	// Get user ID from database
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)
	if err != nil {
		t.Fatalf("Failed to get user ID: %v", err)
	}

	// Login to get token
	loginData := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(loginData))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()

	loginHandler(db).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to login test user: %d", w.Code)
	}

	// Extract token from response
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode login response: %v", err)
	}

	token, ok := resp["token"].(string)
	if !ok {
		t.Fatal("No token in login response")
	}

	return &TestUser{
		ID:       userID,
		Email:    email,
		Password: password,
		Token:    token,
	}
}

func cleanupTestDataForHandler(emails ...string) {
	for _, email := range emails {
		// Clean up profiles first (foreign key constraint)
		db.Exec(`DELETE FROM profiles WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		// Clean up users
		db.Exec("DELETE FROM users WHERE email = $1", email)
	}
}

func getDefaultTestProfileForHandler() *TestProfile {
	return &TestProfile{
		DisplayName:        "Test User",
		AboutMe:            "Test about me",
		ProfilePictureFile: "default.jpg",
		LocationCity:       "Test City",
		LocationLat:        37.7749,
		LocationLon:        -122.4194,
		MaxRadiusKm:        50,
		AnalogPassions:     []string{"reading", "cooking"},
		DigitalDelights:    []string{"coding", "gaming"},
		CrossPollination:   "Looking for creative collaborations",
		FavoriteFood:       "Pizza",
		FavoriteMusic:      "Jazz",
		OtherBio:           map[string]interface{}{"hobby": "photography"},
		MatchPreferences:   map[string]int{"age_min": 25, "age_max": 35},
	}
}

func createTestProfileForHandler(t *testing.T, user *TestUser, profile *TestProfile) {
	profileJSON, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("Failed to marshal profile: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/me/profile", bytes.NewBuffer(profileJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+user.Token)
	w := httptest.NewRecorder()

	completeProfileHandler(db).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Failed to create test profile: %d", w.Code)
	}
}
