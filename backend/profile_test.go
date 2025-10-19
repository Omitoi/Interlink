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

// ============================================================================
// PROFILE MANAGEMENT TEST SUITE
// ============================================================================

func TestProfileManagementSuite(t *testing.T) {
	t.Run("CompleteProfile", func(t *testing.T) {
		testCompleteProfile(t)
	})

	t.Run("ProfileRetrieval", func(t *testing.T) {
		testProfileRetrieval(t)
	})
}

func testCompleteProfile(t *testing.T) {
	tests := []struct {
		name           string
		setupUser      bool
		profile        interface{}
		expectedStatus int
		useValidToken  bool
	}{
		{
			name:           "Successful Profile Completion",
			setupUser:      true,
			profile:        getDefaultTestProfile(),
			expectedStatus: http.StatusOK,
			useValidToken:  true,
		},
		{
			name:           "Unauthorized Access",
			setupUser:      false,
			profile:        getDefaultTestProfile(),
			expectedStatus: http.StatusUnauthorized,
			useValidToken:  false,
		},
		{
			name:           "Invalid JSON",
			setupUser:      true,
			profile:        "{not valid json",
			expectedStatus: http.StatusBadRequest,
			useValidToken:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var user TestUser
			email := fmt.Sprintf("profile_%s@example.com",
				strings.ToLower(strings.ReplaceAll(tt.name, " ", "_")))

			if tt.setupUser {
				defer cleanupTestData(email)
				user = createTestUser(t, email, "password123")
			}

			var reqBody []byte
			var err error

			if str, ok := tt.profile.(string); ok {
				reqBody = []byte(str)
			} else {
				reqBody, err = json.Marshal(tt.profile)
				if err != nil {
					t.Fatalf("failed to marshal profile: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/me/profile", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			if tt.useValidToken && tt.setupUser {
				req.Header.Set("Authorization", "Bearer "+user.Token)
			} else if tt.useValidToken && !tt.setupUser {
				req.Header.Set("Authorization", "Bearer invalid-token")
			}

			w := httptest.NewRecorder()
			completeProfileHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}

	t.Run("Method Not Allowed", func(t *testing.T) {
		email := "profile_method_test@example.com"
		defer cleanupTestData(email)
		user := createTestUser(t, email, "password123")

		req := httptest.NewRequest(http.MethodGet, "/me/profile/complete", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		w := httptest.NewRecorder()

		completeProfileHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

func testProfileRetrieval(t *testing.T) {
	// Setup test users
	userA := createTestUser(t, "profile_ret_a@example.com", "passwordA")
	userB := createTestUser(t, "profile_ret_b@example.com", "passwordB")
	userC := createTestUser(t, "profile_ret_c@example.com", "passwordC")

	defer cleanupTestData(userA.Email, userB.Email, userC.Email)

	// Create profiles
	profileA := getDefaultTestProfile()
	profileA.DisplayName = "User A"
	profileB := getDefaultTestProfile()
	profileB.DisplayName = "User B"
	profileC := getDefaultTestProfile()
	profileC.DisplayName = "User C"
	// Make UserC very different from UserA to ensure no recommendation
	profileC.AnalogPassions = []string{"completely_different_hobby"}
	profileC.DigitalDelights = []string{"completely_different_digital"}
	profileC.FavoriteMusic = "Heavy Metal"
	profileC.LocationLat = -90.0 // Opposite side of the world
	profileC.LocationLon = 180.0

	createTestProfile(t, userA, profileA)
	createTestProfile(t, userB, profileB)
	createTestProfile(t, userC, profileC)

	// Create connection between A and B
	createConnection(t, userA.ID, userB.ID, "accepted")

	tests := []struct {
		name           string
		endpoint       string
		viewerToken    string
		targetUserID   int
		expectedStatus int
	}{
		{
			name:           "Own Profile Access",
			endpoint:       "/me",
			viewerToken:    userA.Token,
			targetUserID:   userA.ID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Own Profile Detail",
			endpoint:       "/me/profile",
			viewerToken:    userA.Token,
			targetUserID:   userA.ID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Own Bio Access",
			endpoint:       "/me/bio",
			viewerToken:    userA.Token,
			targetUserID:   userA.ID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Connected User Profile",
			endpoint:       fmt.Sprintf("/users/%d/profile", userB.ID),
			viewerToken:    userA.Token,
			targetUserID:   userB.ID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Connected User Bio",
			endpoint:       fmt.Sprintf("/users/%d/bio", userB.ID),
			viewerToken:    userA.Token,
			targetUserID:   userB.ID,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Non-Connected User Profile (No Permission)",
			endpoint:       fmt.Sprintf("/users/%d/profile", userC.ID),
			viewerToken:    userA.Token,
			targetUserID:   userC.ID,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Invalid User ID",
			endpoint:       "/users/999999/profile",
			viewerToken:    userA.Token,
			targetUserID:   999999,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Unauthorized Access",
			endpoint:       "/me",
			viewerToken:    "",
			targetUserID:   0,
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.endpoint, nil)

			if tt.viewerToken != "" {
				req.Header.Set("Authorization", "Bearer "+tt.viewerToken)
			}

			w := httptest.NewRecorder()

			// Route to appropriate handler based on endpoint
			switch {
			case tt.endpoint == "/me":
				meHandler(db).ServeHTTP(w, req)
			case tt.endpoint == "/me/profile":
				meProfileHandler(db).ServeHTTP(w, req)
			case tt.endpoint == "/me/bio":
				meBioHandler(db).ServeHTTP(w, req)
			case strings.Contains(tt.endpoint, "/profile"):
				userProfileHandler(db).ServeHTTP(w, req)
			case strings.Contains(tt.endpoint, "/bio"):
				userBioHandler(db).ServeHTTP(w, req)
			default:
				userHandler(db).ServeHTTP(w, req)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Additional validation for successful responses
			if tt.expectedStatus == http.StatusOK {
				var respData map[string]interface{}
				json.NewDecoder(w.Body).Decode(&respData)

				if id, ok := respData["id"].(float64); ok {
					expectedID := tt.targetUserID
					if tt.endpoint == "/me" || tt.endpoint == "/me/profile" || tt.endpoint == "/me/bio" {
						expectedID = userA.ID
					}
					if int(id) != expectedID {
						t.Errorf("expected user ID %d, got %d", expectedID, int(id))
					}
				}
			}
		})
	}
}
