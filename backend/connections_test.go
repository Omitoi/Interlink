package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// CONNECTION SYSTEM TEST SUITE
// ============================================================================

func TestConnectionSystemSuite(t *testing.T) {
	t.Run("ConnectionsHandler", func(t *testing.T) {
		testConnectionsHandler(t)
	})

	t.Run("RequestsHandler", func(t *testing.T) {
		testRequestsHandler(t)
	})

	t.Run("RequestConnectionHandler", func(t *testing.T) {
		testRequestConnectionHandler(t)
	})

	t.Run("AcceptConnectionHandler", func(t *testing.T) {
		testAcceptConnectionHandler(t)
	})

	t.Run("DeclineConnectionHandler", func(t *testing.T) {
		testDeclineConnectionHandler(t)
	})

	t.Run("CancelConnectionRequestHandler", func(t *testing.T) {
		testCancelConnectionRequestHandler(t)
	})

	t.Run("DisconnectConnectionHandler", func(t *testing.T) {
		testDisconnectConnectionHandler(t)
	})

	t.Run("ConnectionsActionsRouter", func(t *testing.T) {
		testConnectionsActionsRouter(t)
	})

	t.Run("ConnectionFlowIntegration", func(t *testing.T) {
		testConnectionFlowIntegration(t)
	})
}

// ============================================================================
// TEST HELPER FUNCTIONS
// ============================================================================

func createTestUserForConnections(t *testing.T, email, password string) TestUser {
	t.Helper()

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

	// Complete profile to make user recommendable - use proper JSON format
	profilePayload := map[string]interface{}{
		"display_name":            fmt.Sprintf("Test User %d", userID),
		"about_me":                "Test bio for connection testing",
		"location_city":           "Test City",
		"location_lat":            60.1699,
		"location_lon":            24.9384,
		"max_radius_km":           50,
		"analog_passions":         []string{"testing", "debugging"},
		"digital_delights":        []string{"coding", "apis"},
		"collaboration_interests": "Looking for test connections",
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

	return TestUser{
		ID:    userID,
		Email: email,
		Token: token,
	}
}

func cleanupConnectionTestData(userEmails ...string) {
	for _, email := range userEmails {
		var userID int
		db.QueryRow("SELECT id FROM users WHERE email = $1", email).Scan(&userID)

		// Clean up connections first (foreign key constraints)
		db.Exec("DELETE FROM connections WHERE user_id = $1 OR target_user_id = $1", userID)

		// Clean up user data
		db.Exec("DELETE FROM profiles WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE email = $1", email)
	}
}

// ============================================================================
// CONNECTIONS HANDLER TESTS
// ============================================================================

func testConnectionsHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "conn_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "conn_test_b@example.com", "password123")
	userC := createTestUserForConnections(t, "conn_test_c@example.com", "password123")

	defer cleanupConnectionTestData("conn_test_a@example.com", "conn_test_b@example.com", "conn_test_c@example.com")

	// Create some accepted connections for userA
	db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
		VALUES ($1, $2, 'accepted', NOW())`, userA.ID, userB.ID)
	db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
		VALUES ($1, $2, 'accepted', NOW())`, userA.ID, userC.ID)

	tests := []struct {
		name           string
		token          string
		expectedStatus int
		expectedCount  int
		description    string
	}{
		{
			name:           "Valid Request - Has Connections",
			token:          userA.Token,
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			description:    "Should return list of accepted connections",
		},
		{
			name:           "Valid Request - No Connections",
			token:          userB.Token,
			expectedStatus: http.StatusOK,
			expectedCount:  1, // userB is connected to userA
			description:    "Should return empty list for user with no connections",
		},
		{
			name:           "Unauthenticated Request",
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
			description:    "Should reject request without auth token",
		},
		{
			name:           "Invalid Token",
			token:          "invalid_token_here",
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
			description:    "Should reject request with invalid token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/connections", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			connectionsHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp map[string][]int
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				connections := resp["connections"]
				if len(connections) != tt.expectedCount {
					t.Errorf("Expected %d connections, got %d", tt.expectedCount, len(connections))
				}
			}
		})
	}
}

// ============================================================================
// REQUESTS HANDLER TESTS
// ============================================================================

func testRequestsHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "req_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "req_test_b@example.com", "password123")
	userC := createTestUserForConnections(t, "req_test_c@example.com", "password123")

	defer cleanupConnectionTestData("req_test_a@example.com", "req_test_b@example.com", "req_test_c@example.com")

	// Create some pending requests TO userA
	db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
		VALUES ($1, $2, 'pending', NOW())`, userB.ID, userA.ID)
	db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
		VALUES ($1, $2, 'pending', NOW())`, userC.ID, userA.ID)

	tests := []struct {
		name           string
		method         string
		token          string
		expectedStatus int
		expectedCount  int
		description    string
	}{
		{
			name:           "Valid GET Request - Has Pending Requests",
			method:         http.MethodGet,
			token:          userA.Token,
			expectedStatus: http.StatusOK,
			expectedCount:  2,
			description:    "Should return pending requests to user",
		},
		{
			name:           "Valid GET Request - No Pending Requests",
			method:         http.MethodGet,
			token:          userB.Token,
			expectedStatus: http.StatusOK,
			expectedCount:  0,
			description:    "Should return empty list for user with no pending requests",
		},
		{
			name:           "Invalid Method",
			method:         http.MethodPost,
			token:          userA.Token,
			expectedStatus: http.StatusMethodNotAllowed,
			expectedCount:  0,
			description:    "Should reject non-GET requests",
		},
		{
			name:           "Unauthenticated Request",
			method:         http.MethodGet,
			token:          "",
			expectedStatus: http.StatusUnauthorized,
			expectedCount:  0,
			description:    "Should reject unauthenticated requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/requests", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			requestsHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK {
				var resp map[string][]int
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				requests := resp["requests"]
				if len(requests) != tt.expectedCount {
					t.Errorf("Expected %d requests, got %d", tt.expectedCount, len(requests))
				}
			}
		})
	}
}

// ============================================================================
// REQUEST CONNECTION HANDLER TESTS
// ============================================================================

func testRequestConnectionHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "request_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "request_test_b@example.com", "password123")

	defer cleanupConnectionTestData("request_test_a@example.com", "request_test_b@example.com")

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		targetUser     TestUser
		setupFunc      func()
		expectedStatus int
		expectedResult string
		description    string
	}{
		{
			name:           "Valid Connection Request",
			method:         http.MethodPost,
			path:           fmt.Sprintf("/connections/%d/request", userB.ID),
			token:          userA.Token,
			targetUser:     userB,
			setupFunc:      func() {},
			expectedStatus: http.StatusOK,
			expectedResult: "pending",
			description:    "Should create new connection request",
		},
		{
			name:           "Request to Self",
			method:         http.MethodPost,
			path:           fmt.Sprintf("/connections/%d/request", userA.ID),
			token:          userA.Token,
			targetUser:     userA,
			setupFunc:      func() {},
			expectedStatus: http.StatusBadRequest,
			expectedResult: "invalid_target",
			description:    "Should reject request to connect to self",
		},
		{
			name:           "Invalid Method",
			method:         http.MethodGet,
			path:           fmt.Sprintf("/connections/%d/request", userB.ID),
			token:          userA.Token,
			targetUser:     userB,
			setupFunc:      func() {},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedResult: "invalid_method",
			description:    "Should reject non-POST requests",
		},
		{
			name:           "Invalid Target ID",
			method:         http.MethodPost,
			path:           "/connections/invalid/request",
			token:          userA.Token,
			targetUser:     userB,
			setupFunc:      func() {},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject invalid target ID",
		},
		{
			name:           "Unauthenticated Request",
			method:         http.MethodPost,
			path:           fmt.Sprintf("/connections/%d/request", userB.ID),
			token:          "",
			targetUser:     userB,
			setupFunc:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedResult: "",
			description:    "Should reject unauthenticated requests",
		},
		{
			name:       "Duplicate Request",
			method:     http.MethodPost,
			path:       fmt.Sprintf("/connections/%d/request", userB.ID),
			token:      userA.Token,
			targetUser: userB,
			setupFunc: func() {
				// Debug: Check if target user exists and is complete
				var exists bool
				err := db.QueryRow(`
					SELECT EXISTS (
						SELECT 1
						FROM users u
						JOIN profiles p ON p.user_id = u.id
						WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
					)
				`, userB.ID).Scan(&exists)
				if err != nil || !exists {
					t.Logf("Target user %d does not exist or is incomplete: err=%v, exists=%v", userB.ID, err, exists)
				}

				// Create existing pending request
				result, err := db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW())`, userA.ID, userB.ID)
				if err != nil {
					t.Logf("Failed to create connection: %v", err)
				} else {
					affected, _ := result.RowsAffected()
					t.Logf("Created connection: affected rows=%d", affected)
				}
			},
			expectedStatus: http.StatusOK,
			expectedResult: "pending",
			description:    "Should return existing pending request",
		},
		{
			name:           "Invalid URL Format",
			method:         http.MethodPost,
			path:           "/connections/request/wrong",
			token:          userA.Token,
			targetUser:     userB,
			setupFunc:      func() {},
			expectedStatus: http.StatusNotFound,
			expectedResult: "",
			description:    "Should reject invalid URL paths",
		},
		{
			name:           "Target User Not Found",
			method:         http.MethodPost,
			path:           "/connections/999999/request",
			token:          userA.Token,
			targetUser:     userB,
			setupFunc:      func() {},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when target user doesn't exist",
		},
		{
			name:       "Auto-Accept Mutual Request",
			method:     http.MethodPost,
			path:       fmt.Sprintf("/connections/%d/request", userB.ID),
			token:      userA.Token,
			targetUser: userB,
			setupFunc: func() {
				// Create pending request from userB to userA (opposite direction)
				db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW())`, userB.ID, userA.ID)
			},
			expectedStatus: http.StatusOK,
			expectedResult: "accepted",
			description:    "Should auto-accept when mutual request exists",
		},
		{
			name:       "Already Accepted Connection",
			method:     http.MethodPost,
			path:       fmt.Sprintf("/connections/%d/request", userB.ID),
			token:      userA.Token,
			targetUser: userB,
			setupFunc: func() {
				// Create accepted connection between users
				db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'accepted', NOW())`, userA.ID, userB.ID)
			},
			expectedStatus: http.StatusOK,
			expectedResult: "accepted",
			description:    "Should return accepted idempotently",
		},
		{
			name:       "Previously Dismissed Connection",
			method:     http.MethodPost,
			path:       fmt.Sprintf("/connections/%d/request", userB.ID),
			token:      userA.Token,
			targetUser: userB,
			setupFunc: func() {
				// Create dismissed connection between users
				db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'dismissed', NOW())`, userA.ID, userB.ID)
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject request to previously dismissed connection",
		},
		{
			name:       "Previously Disconnected Connection",
			method:     http.MethodPost,
			path:       fmt.Sprintf("/connections/%d/request", userB.ID),
			token:      userA.Token,
			targetUser: userB,
			setupFunc: func() {
				// Create disconnected connection between users
				db.Exec(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'disconnected', NOW())`, userA.ID, userB.ID)
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject request to previously disconnected connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing connections
			db.Exec("DELETE FROM connections WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)",
				userA.ID, tt.targetUser.ID)

			// Run setup function
			tt.setupFunc()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			requestConnectionHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedResult != "" && tt.expectedStatus != http.StatusUnauthorized {
				// Handle different response formats
				if tt.expectedStatus >= 400 {
					var resp map[string]string
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
						t.Fatalf("Failed to parse error response: %v", err)
					}
					if resp["error"] != tt.expectedResult {
						t.Errorf("Expected error '%s', got '%s'", tt.expectedResult, resp["error"])
					}
				} else {
					// Success response format: {"state":"pending","connection_id":123}
					var resp map[string]interface{}
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
						t.Fatalf("Failed to parse success response: %v", err)
					}
					if resp["state"] != tt.expectedResult {
						t.Errorf("Expected state '%s', got '%s'", tt.expectedResult, resp["state"])
					}
					// Verify connection_id is present for successful requests
					if _, ok := resp["connection_id"]; !ok {
						t.Errorf("Expected connection_id in response")
					}
				}
			}
		})
	}
}

// ============================================================================
// ACCEPT CONNECTION HANDLER TESTS
// ============================================================================

func testAcceptConnectionHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "accept_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "accept_test_b@example.com", "password123")

	defer cleanupConnectionTestData("accept_test_a@example.com", "accept_test_b@example.com")

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		setupFunc      func() int
		expectedStatus int
		expectedResult string
		description    string
	}{
		{
			name:   "Valid Accept Request",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending request from userB to userA
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusOK,
			expectedResult: "accepted",
			description:    "Should accept pending connection request",
		},
		{
			name:   "No Pending Request",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when no pending request exists",
		},
		{
			name:   "Invalid Method",
			method: http.MethodGet,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedResult: "invalid_method",
			description:    "Should reject non-POST requests",
		},
		{
			name:   "Unauthenticated Request",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  "",
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusUnauthorized,
			expectedResult: "",
			description:    "Should reject unauthenticated requests",
		},
		{
			name:   "Invalid URL Format",
			method: http.MethodPost,
			path:   "/connections/accept/wrong",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "",
			description:    "Should reject invalid URL paths",
		},
		{
			name:   "Invalid User ID Format",
			method: http.MethodPost,
			path:   "/connections/invalid/accept",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject non-numeric user IDs",
		},
		{
			name:   "Self Target ID",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userA.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusBadRequest,
			expectedResult: "invalid_target",
			description:    "Should reject when target ID equals requester ID",
		},
		{
			name:   "Target User Not Found",
			method: http.MethodPost,
			path:   "/connections/999999/accept",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when target user doesn't exist",
		},
		{
			name:   "Own Pending Request (Nothing to Accept)",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending request FROM userA TO userB (user's own request)
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when trying to accept own outgoing request",
		},
		{
			name:   "Already Accepted (Idempotent)",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create accepted connection from userB to userA
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'accepted', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusOK,
			expectedResult: "accepted",
			description:    "Should return accepted idempotently",
		},
		{
			name:   "Already Dismissed Connection",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create dismissed connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'dismissed', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject accepting a dismissed connection",
		},
		{
			name:   "Disconnected Connection",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/accept", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create disconnected connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'disconnected', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject accepting a disconnected connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing connections
			db.Exec("DELETE FROM connections WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)",
				userA.ID, userB.ID)

			// Run setup function
			tt.setupFunc()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			acceptConnectionHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedResult != "" && tt.expectedStatus != http.StatusUnauthorized {
				// Handle different response formats
				if tt.expectedStatus >= 400 {
					var resp map[string]string
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
						t.Fatalf("Failed to parse error response: %v", err)
					}
					if resp["error"] != tt.expectedResult {
						t.Errorf("Expected error '%s', got '%s'", tt.expectedResult, resp["error"])
					}
				} else {
					// Success response format: {"state":"accepted","connection_id":123}
					var resp map[string]interface{}
					if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
						t.Fatalf("Failed to parse success response: %v", err)
					}
					if resp["state"] != tt.expectedResult {
						t.Errorf("Expected state '%s', got '%s'", tt.expectedResult, resp["state"])
					}
					// Verify connection_id is present for successful requests
					if _, ok := resp["connection_id"]; !ok {
						t.Errorf("Expected connection_id in response")
					}
				}
			}
		})
	}
}

// ============================================================================
// DECLINE CONNECTION HANDLER TESTS
// ============================================================================

func testDeclineConnectionHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "decline_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "decline_test_b@example.com", "password123")

	defer cleanupConnectionTestData("decline_test_a@example.com", "decline_test_b@example.com")

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		setupFunc      func() int
		expectedStatus int
		expectedResult string
		description    string
	}{
		{
			name:   "Valid Decline Request",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending request from userB to userA
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusOK,
			expectedResult: "dismissed",
			description:    "Should decline pending connection request",
		},
		{
			name:   "No Pending Request",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when no pending request exists",
		},
		{
			name:   "Invalid Method",
			method: http.MethodGet,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedResult: "invalid_method",
			description:    "Should reject non-GET requests",
		},
		{
			name:   "Invalid URL Format",
			method: http.MethodPost,
			path:   "/connections/decline/wrong",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "",
			description:    "Should reject invalid URL paths",
		},
		{
			name:   "Invalid User ID Format",
			method: http.MethodPost,
			path:   "/connections/invalid/decline",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject non-numeric user IDs",
		},
		{
			name:   "Self Target ID",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userA.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusBadRequest,
			expectedResult: "invalid_target",
			description:    "Should reject when target ID equals requester ID",
		},
		{
			name:   "Target User Not Found",
			method: http.MethodPost,
			path:   "/connections/999999/decline",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when target user doesn't exist",
		},
		{
			name:   "Own Pending Request (Nothing to Decline)",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending request FROM userA TO userB (user's own request)
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when trying to decline own outgoing request",
		},
		{
			name:   "Already Dismissed (Idempotent)",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create dismissed request from userB to userA
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'dismissed', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusOK,
			expectedResult: "dismissed",
			description:    "Should return dismissed idempotently",
		},
		{
			name:   "Already Accepted Connection",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create accepted connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'accepted', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject declining an accepted connection",
		},
		{
			name:   "Disconnected Connection",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/decline", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create disconnected connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'disconnected', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject declining a disconnected connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing connections
			db.Exec("DELETE FROM connections WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)",
				userA.ID, userB.ID)

			// Run setup function
			tt.setupFunc()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			declineConnectionHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedResult != "" && tt.expectedStatus != http.StatusUnauthorized {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if tt.expectedStatus >= 400 {
					if resp["error"] != tt.expectedResult {
						t.Errorf("Expected error '%s', got '%s'", tt.expectedResult, resp["error"])
					}
				} else {
					if resp["state"] != tt.expectedResult {
						t.Errorf("Expected state '%s', got '%s'", tt.expectedResult, resp["state"])
					}
				}
			}
		})
	}
}

// ============================================================================
// CANCEL CONNECTION REQUEST HANDLER TESTS
// ============================================================================

func testCancelConnectionRequestHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "cancel_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "cancel_test_b@example.com", "password123")

	defer cleanupConnectionTestData("cancel_test_a@example.com", "cancel_test_b@example.com")

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		setupFunc      func() int
		expectedStatus int
		expectedResult string
		description    string
	}{
		{
			name:   "Valid Cancel Request",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending request from userA to userB
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusOK,
			expectedResult: "dismissed",
			description:    "Should cancel user's own pending request",
		},
		{
			name:   "No Pending Request to Cancel",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when no pending request exists",
		},
		{
			name:   "Invalid Method",
			method: http.MethodGet,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedResult: "invalid_method",
			description:    "Should reject non-POST requests",
		},
		{
			name:   "Invalid URL Format",
			method: http.MethodPost,
			path:   "/connections/cancel/wrong",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "",
			description:    "Should reject invalid URL paths",
		},
		{
			name:   "Invalid User ID Format",
			method: http.MethodPost,
			path:   "/connections/invalid/cancel",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject non-numeric user IDs",
		},
		{
			name:   "Self Target ID",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userA.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusBadRequest,
			expectedResult: "invalid_target",
			description:    "Should reject when target ID equals requester ID",
		},
		{
			name:   "Target User Not Found",
			method: http.MethodPost,
			path:   "/connections/999999/cancel",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when target user doesn't exist",
		},
		{
			name:   "Try Cancel Incoming Request (Not Yours)",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending request FROM userB TO userA (not userA's to cancel)
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userB.ID, userA.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when trying to cancel incoming request",
		},
		{
			name:   "Already Dismissed (Idempotent)",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create dismissed request from userA to userB
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'dismissed', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusOK,
			expectedResult: "dismissed",
			description:    "Should return dismissed idempotently",
		},
		{
			name:   "Already Accepted Connection",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create accepted connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'accepted', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject canceling an accepted connection",
		},
		{
			name:   "Disconnected Connection",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d/cancel", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create disconnected connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'disconnected', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject canceling a disconnected connection",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing connections
			db.Exec("DELETE FROM connections WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)",
				userA.ID, userB.ID)

			// Run setup function
			tt.setupFunc()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			cancelConnectionRequestHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedResult != "" && tt.expectedStatus != http.StatusUnauthorized {
				var resp map[string]interface{}
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if tt.expectedStatus >= 400 {
					if resp["error"] != tt.expectedResult {
						t.Errorf("Expected error '%s', got '%s'", tt.expectedResult, resp["error"])
					}
				} else {
					if resp["state"] != tt.expectedResult {
						t.Errorf("Expected state '%s', got '%s'", tt.expectedResult, resp["state"])
					}
				}
			}
		})
	}
}

// ============================================================================
// DISCONNECT CONNECTION HANDLER TESTS
// ============================================================================

func testDisconnectConnectionHandler(t *testing.T) {
	userA := createTestUserForConnections(t, "disconnect_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "disconnect_test_b@example.com", "password123")

	defer cleanupConnectionTestData("disconnect_test_a@example.com", "disconnect_test_b@example.com")

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		setupFunc      func() int
		expectedStatus int
		expectedResult string
		description    string
	}{
		{
			name:   "Valid Disconnect Request",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create accepted connection between userA and userB
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'accepted', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusNoContent,
			expectedResult: "",
			description:    "Should disconnect existing accepted connection",
		},
		{
			name:   "No Connection to Disconnect",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when no connection exists",
		},
		{
			name:   "Invalid Method",
			method: http.MethodPost,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusMethodNotAllowed,
			expectedResult: "invalid_method",
			description:    "Should reject non-DELETE requests",
		},
		{
			name:   "Invalid URL Format",
			method: http.MethodDelete,
			path:   "/connections/disconnect/wrong",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "",
			description:    "Should reject invalid URL paths",
		},
		{
			name:   "Invalid User ID Format",
			method: http.MethodDelete,
			path:   "/connections/invalid",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject non-numeric user IDs",
		},
		{
			name:   "Self Target ID",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userA.ID),
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusBadRequest,
			expectedResult: "invalid_target",
			description:    "Should reject when target ID equals requester ID",
		},
		{
			name:   "Target User Not Found",
			method: http.MethodDelete,
			path:   "/connections/999999",
			token:  userA.Token,
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusNotFound,
			expectedResult: "not_found",
			description:    "Should reject when target user doesn't exist",
		},
		{
			name:   "Already Disconnected (Idempotent)",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create disconnected connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'disconnected', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusNoContent,
			expectedResult: "",
			description:    "Should return No Content idempotently for already disconnected",
		},
		{
			name:   "Pending Connection",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create pending connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'pending', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject disconnecting a pending connection",
		},
		{
			name:   "Dismissed Connection",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  userA.Token,
			setupFunc: func() int {
				// Create dismissed connection between users
				var connID int
				db.QueryRow(`INSERT INTO connections (user_id, target_user_id, status, created_at) 
					VALUES ($1, $2, 'dismissed', NOW()) RETURNING id`,
					userA.ID, userB.ID).Scan(&connID)
				return connID
			},
			expectedStatus: http.StatusConflict,
			expectedResult: "invalid_state",
			description:    "Should reject disconnecting a dismissed connection",
		},
		{
			name:   "Unauthenticated Request",
			method: http.MethodDelete,
			path:   fmt.Sprintf("/connections/%d", userB.ID),
			token:  "",
			setupFunc: func() int {
				return 0
			},
			expectedStatus: http.StatusUnauthorized,
			expectedResult: "",
			description:    "Should reject unauthenticated requests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing connections
			db.Exec("DELETE FROM connections WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)",
				userA.ID, userB.ID)

			// Run setup function
			tt.setupFunc()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			w := httptest.NewRecorder()

			disconnectConnectionHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d. Body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if tt.expectedResult != "" && tt.expectedStatus != http.StatusUnauthorized {
				var resp map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
					t.Fatalf("Failed to parse response: %v", err)
				}

				if tt.expectedStatus >= 400 {
					if resp["error"] != tt.expectedResult {
						t.Errorf("Expected error '%s', got '%s'", tt.expectedResult, resp["error"])
					}
				} else {
					if resp["status"] != tt.expectedResult {
						t.Errorf("Expected status '%s', got '%s'", tt.expectedResult, resp["status"])
					}
				}
			}
		})
	}
}

// ============================================================================
// CONNECTIONS ACTIONS ROUTER TESTS
// ============================================================================

func testConnectionsActionsRouter(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
		description    string
	}{
		{
			name:           "Valid Request Route",
			method:         http.MethodPost,
			path:           "/connections/123/request",
			expectedStatus: http.StatusUnauthorized, // Will fail auth but route correctly
			description:    "Should route to requestConnectionHandler",
		},
		{
			name:           "Valid Accept Route",
			method:         http.MethodPost,
			path:           "/connections/123/accept",
			expectedStatus: http.StatusUnauthorized, // Will fail auth but route correctly
			description:    "Should route to acceptConnectionHandler",
		},
		{
			name:           "Valid Decline Route",
			method:         http.MethodPost,
			path:           "/connections/123/decline",
			expectedStatus: http.StatusUnauthorized, // Will fail auth but route correctly
			description:    "Should route to declineConnectionHandler",
		},
		{
			name:           "Valid Cancel Route",
			method:         http.MethodPost,
			path:           "/connections/123/cancel",
			expectedStatus: http.StatusUnauthorized, // Will fail auth but route correctly
			description:    "Should route to cancelConnectionRequestHandler",
		},
		{
			name:           "Valid Disconnect Route",
			method:         http.MethodDelete,
			path:           "/connections/123",
			expectedStatus: http.StatusUnauthorized, // Will fail auth but route correctly
			description:    "Should route to disconnectConnectionHandler",
		},
		{
			name:           "Invalid Action",
			method:         http.MethodPost,
			path:           "/connections/123/invalid",
			expectedStatus: http.StatusNotFound,
			description:    "Should return 404 for invalid actions",
		},
		{
			name:           "Invalid Path Structure",
			method:         http.MethodPost,
			path:           "/invalid/123/request",
			expectedStatus: http.StatusNotFound,
			description:    "Should return 404 for invalid paths",
		},
		{
			name:           "Too Many Path Segments",
			method:         http.MethodPost,
			path:           "/connections/123/request/extra",
			expectedStatus: http.StatusNotFound,
			description:    "Should return 404 for paths with extra segments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			connectionsActionsRouter(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d for path %s", tt.expectedStatus, w.Code, tt.path)
			}
		})
	}
}

// ============================================================================
// INTEGRATION TEST - FULL CONNECTION FLOW
// ============================================================================

func testConnectionFlowIntegration(t *testing.T) {
	userA := createTestUserForConnections(t, "flow_test_a@example.com", "password123")
	userB := createTestUserForConnections(t, "flow_test_b@example.com", "password123")

	defer cleanupConnectionTestData("flow_test_a@example.com", "flow_test_b@example.com")

	t.Run("Complete Connection Flow", func(t *testing.T) {
		// 1. UserA sends connection request to UserB
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/connections/%d/request", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w := httptest.NewRecorder()

		requestConnectionHandler(db).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for connection request, got %d", w.Code)
		}

		// 2. UserB should see the pending request
		req = httptest.NewRequest(http.MethodGet, "/requests", nil)
		req.Header.Set("Authorization", "Bearer "+userB.Token)
		w = httptest.NewRecorder()

		requestsHandler(db).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for requests list, got %d", w.Code)
		}

		var reqResp map[string][]int
		json.Unmarshal(w.Body.Bytes(), &reqResp)
		if len(reqResp["requests"]) != 1 {
			t.Fatalf("Expected 1 pending request, got %d", len(reqResp["requests"]))
		}
		if reqResp["requests"][0] != userA.ID {
			t.Fatalf("Expected request from user %d, got %d", userA.ID, reqResp["requests"][0])
		}

		// 3. UserB accepts the connection request
		req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/connections/%d/accept", userA.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userB.Token)
		w = httptest.NewRecorder()

		acceptConnectionHandler(db).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for accept connection, got %d", w.Code)
		}

		// 4. Both users should see each other in their connections
		req = httptest.NewRequest(http.MethodGet, "/connections", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w = httptest.NewRecorder()

		connectionsHandler(db).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for connections list, got %d", w.Code)
		}

		var connResp map[string][]int
		json.Unmarshal(w.Body.Bytes(), &connResp)
		if len(connResp["connections"]) != 1 {
			t.Fatalf("Expected 1 connection for userA, got %d", len(connResp["connections"]))
		}
		if connResp["connections"][0] != userB.ID {
			t.Fatalf("Expected connection to user %d, got %d", userB.ID, connResp["connections"][0])
		}

		// 5. UserA disconnects
		req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/connections/%d", userB.ID), nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w = httptest.NewRecorder()

		disconnectConnectionHandler(db).ServeHTTP(w, req)
		if w.Code != http.StatusNoContent {
			t.Fatalf("Expected 204 for disconnect, got %d", w.Code)
		}

		// 6. Both users should have no connections now
		req = httptest.NewRequest(http.MethodGet, "/connections", nil)
		req.Header.Set("Authorization", "Bearer "+userA.Token)
		w = httptest.NewRecorder()

		connectionsHandler(db).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("Expected 200 for connections list after disconnect, got %d", w.Code)
		}

		json.Unmarshal(w.Body.Bytes(), &connResp)
		if len(connResp["connections"]) != 0 {
			t.Fatalf("Expected 0 connections after disconnect, got %d", len(connResp["connections"]))
		}
	})
}
