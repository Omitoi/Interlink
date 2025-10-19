package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============================================================================
// ERROR HANDLING TEST SUITE
// ============================================================================

func TestErrorHandlingSuite(t *testing.T) {
	t.Run("AuthorizationErrors", func(t *testing.T) {
		testAuthorizationErrors(t)
	})

	t.Run("ValidationErrors", func(t *testing.T) {
		testValidationErrors(t)
	})
}

func testAuthorizationErrors(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		token    string
		method   string
	}{
		{"No Token - ME", "/me", "", http.MethodGet},
		{"Invalid Token - ME", "/me", "invalid", http.MethodGet},
		{"No Token - Recommendations", "/recommendations", "", http.MethodGet},
		{"Invalid Token - Recommendations", "/recommendations", "invalid", http.MethodGet},
		{"No Token - Connections", "/connections", "", http.MethodGet},
		{"Invalid Token - Connections", "/connections", "invalid", http.MethodGet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.endpoint, nil)

			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			w := httptest.NewRecorder()

			// Route to appropriate handler
			switch tt.endpoint {
			case "/me":
				meHandler(db).ServeHTTP(w, req)
			case "/recommendations":
				recommendationsHandler(db).ServeHTTP(w, req)
			case "/connections":
				connectionsHandler(db).ServeHTTP(w, req)
			}

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected status 401, got %d", w.Code)
			}
		})
	}
}

func testValidationErrors(t *testing.T) {
	email := "validation_test@example.com"
	defer cleanupTestData(email)

	user := createTestUser(t, email, "password123")

	tests := []struct {
		name           string
		endpoint       string
		method         string
		body           string
		expectedStatus int
	}{
		{
			name:           "Invalid JSON in Profile",
			endpoint:       "/me/profile",
			method:         http.MethodPost,
			body:           "{invalid json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Wrong Method for Profile",
			endpoint:       "/me/profile/complete",
			method:         http.MethodGet,
			body:           "",
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.endpoint, bytes.NewBufferString(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.endpoint, nil)
			}

			req.Header.Set("Authorization", "Bearer "+user.Token)
			w := httptest.NewRecorder()

			completeProfileHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
