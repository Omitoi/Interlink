package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================================================
// MIDDLEWARE AND ROUTING TEST SUITE
// ============================================================================

func TestMiddlewareAndRoutingSuite(t *testing.T) {
	t.Run("CORS", func(t *testing.T) {
		testCORS(t)
	})

	t.Run("URLRouting", func(t *testing.T) {
		testURLRouting(t)
	})
}

func testCORS(t *testing.T) {
	t.Run("CORS Headers Applied", func(t *testing.T) {
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusTeapot)
		})

		req := httptest.NewRequest(http.MethodGet, "/anything", nil)
		req.Header.Set("Origin", "http://127.0.0.1:5173") // Set Origin header
		w := httptest.NewRecorder()

		withCORS(handler).ServeHTTP(w, req)

		resp := w.Result()

		if resp.Header.Get("Access-Control-Allow-Origin") != "http://127.0.0.1:5173" {
			t.Errorf("missing or wrong CORS origin header: %v",
				resp.Header.Get("Access-Control-Allow-Origin"))
		}

		if !called {
			t.Error("expected wrapped handler to be called")
		}

		if resp.StatusCode != http.StatusTeapot {
			t.Errorf("expected status %d, got %d", http.StatusTeapot, resp.StatusCode)
		}
	})

	t.Run("OPTIONS Preflight", func(t *testing.T) {
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})

		req := httptest.NewRequest(http.MethodOptions, "/anything", nil)
		w := httptest.NewRecorder()

		withCORS(handler).ServeHTTP(w, req)

		resp := w.Result()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status %d for OPTIONS, got %d",
				http.StatusNoContent, resp.StatusCode)
		}

		if called {
			t.Error("handler should not be called for OPTIONS preflight")
		}
	})
}

func testURLRouting(t *testing.T) {
	// Setup test users with connection for permission testing
	userA := createTestUser(t, "routing_a@example.com", "passwordA")
	userB := createTestUser(t, "routing_b@example.com", "passwordB")

	defer cleanupTestData(userA.Email, userB.Email)

	// Create profiles to ensure endpoints work
	profileA := getDefaultTestProfile()
	profileA.DisplayName = "Routing A"
	profileB := getDefaultTestProfile()
	profileB.DisplayName = "Routing B"

	createTestProfile(t, userA, profileA)
	createTestProfile(t, userB, profileB)

	// Create connection for permission
	createConnection(t, userA.ID, userB.ID, "accepted")

	tests := []struct {
		name           string
		method         string
		path           string
		token          string
		expectedStatus int
	}{
		{
			name:           "User Summary Route",
			method:         http.MethodGet,
			path:           fmt.Sprintf("/users/%d", userB.ID),
			token:          userA.Token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "User Profile Route",
			method:         http.MethodGet,
			path:           fmt.Sprintf("/users/%d/profile", userB.ID),
			token:          userA.Token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "User Bio Route",
			method:         http.MethodGet,
			path:           fmt.Sprintf("/users/%d/bio", userB.ID),
			token:          userA.Token,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Invalid Route",
			method:         http.MethodGet,
			path:           fmt.Sprintf("/users/%d/unknown", userB.ID),
			token:          userA.Token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Malformed User ID in Profile",
			method:         http.MethodGet,
			path:           "/users/notanint/profile",
			token:          userA.Token,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Malformed User ID in Bio",
			method:         http.MethodGet,
			path:           "/users/xyz/bio",
			token:          userA.Token,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)

			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			w := httptest.NewRecorder()

			// Route through the dispatcher
			if strings.HasPrefix(tt.path, "/users/") {
				usersDispatcher(db).ServeHTTP(w, req)
			} else {
				// Handle other routes as needed
				w.WriteHeader(http.StatusNotFound)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}
