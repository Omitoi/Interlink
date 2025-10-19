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

// Initialize JWT secret for auth tests
func init() {
	jwtSecret = []byte("test-secret-key-for-testing")
}

// ============================================================================
// AUTHENTICATION TEST SUITE
// ============================================================================

func TestAuthenticationSuite(t *testing.T) {
	t.Run("Registration", func(t *testing.T) {
		testRegistration(t)
	})

	t.Run("Login", func(t *testing.T) {
		testLogin(t)
	})
}

func testRegistration(t *testing.T) {
	tests := []struct {
		name           string
		email          string
		password       string
		setupFunc      func(string)
		expectedStatus int
		cleanup        bool
	}{
		{
			name:           "Valid Registration",
			email:          "testuser_valid@example.com",
			password:       "testpass123",
			setupFunc:      func(email string) { db.Exec("DELETE FROM users WHERE email = $1", email) },
			expectedStatus: http.StatusCreated,
			cleanup:        true,
		},
		{
			name:     "Duplicate Email",
			email:    "testuser_duplicate@example.com",
			password: "anotherpass",
			setupFunc: func(email string) {
				db.Exec("DELETE FROM users WHERE email = $1", email)
				hash, _ := bcrypt.GenerateFromPassword([]byte("somepassword"), bcrypt.DefaultCost)
				db.Exec("INSERT INTO users (email, password_hash) VALUES ($1, $2)", email, string(hash))
			},
			expectedStatus: http.StatusConflict,
			cleanup:        true,
		},
		{
			name:           "Invalid JSON",
			email:          "",
			password:       "",
			setupFunc:      func(string) {},
			expectedStatus: http.StatusBadRequest,
			cleanup:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cleanup {
				defer cleanupTestData(tt.email)
			}

			tt.setupFunc(tt.email)

			var reqBody []byte
			if tt.name == "Invalid JSON" {
				reqBody = []byte(`{not valid json}`)
			} else {
				reqBody = []byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, tt.email, tt.password))
			}

			req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			registerHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}

	t.Run("Invalid Method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/register", nil)
		w := httptest.NewRecorder()
		registerHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

func testLogin(t *testing.T) {
	// Setup test user
	email := "login_test@example.com"
	password := "testpass123"
	defer cleanupTestData(email)

	user := createTestUser(t, email, password)

	tests := []struct {
		name           string
		email          string
		password       string
		expectedStatus int
		expectToken    bool
	}{
		{
			name:           "Successful Login",
			email:          user.Email,
			password:       user.Password,
			expectedStatus: http.StatusOK,
			expectToken:    true,
		},
		{
			name:           "Wrong Password",
			email:          user.Email,
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name:           "Nonexistent User",
			email:          "doesnotexist@example.com",
			password:       "irrelevant",
			expectedStatus: http.StatusUnauthorized,
			expectToken:    false,
		},
		{
			name:           "Missing Fields",
			email:          "",
			password:       "",
			expectedStatus: http.StatusBadRequest,
			expectToken:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := []byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, tt.email, tt.password))
			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			loginHandler(db).ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectToken {
				var respBody map[string]string
				json.NewDecoder(w.Body).Decode(&respBody)
				if _, ok := respBody["token"]; !ok {
					t.Error("expected token in response")
				}
			}
		})
	}

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer([]byte(`{not valid json}`)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		loginHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}
