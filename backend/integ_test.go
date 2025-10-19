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
// INTEGRATION TEST SUITE
// ============================================================================

func TestIntegrationSuite(t *testing.T) {
	t.Run("EndToEndUserFlow", func(t *testing.T) {
		testEndToEndUserFlow(t)
	})
}

func testEndToEndUserFlow(t *testing.T) {
	// This test demonstrates a complete user journey
	t.Run("Complete User Journey", func(t *testing.T) {
		email := "integration_user@example.com"
		password := "password123"

		defer cleanupTestData(email)

		// Step 1: Register
		reqBody := []byte(fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password))
		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewBuffer(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		registerHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("registration failed: %d", w.Code)
		}

		// Step 2: Login
		token := loginUser(t, email, password)

		// Step 3: Complete Profile
		profile := getDefaultTestProfile()
		profile.DisplayName = "Integration Test User"

		profileJSON, _ := json.Marshal(profile)
		req = httptest.NewRequest(http.MethodPost, "/me/profile", bytes.NewBuffer(profileJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		completeProfileHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("profile completion failed: %d", w.Code)
		}

		// Step 4: Access Own Profile
		req = httptest.NewRequest(http.MethodGet, "/me", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		meHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("profile access failed: %d", w.Code)
		}

		var profileResp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&profileResp)

		if profileResp["display_name"] != "Integration Test User" {
			t.Errorf("expected display_name 'Integration Test User', got %v",
				profileResp["display_name"])
		}

		// Step 5: Request Recommendations (should work now with complete profile)
		req = httptest.NewRequest(http.MethodGet, "/recommendations", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w = httptest.NewRecorder()

		recommendationsHandler(db).ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("recommendations failed: %d", w.Code)
		}

		var recResp struct {
			Recommendations []int `json:"recommendations"`
		}
		json.NewDecoder(w.Body).Decode(&recResp)

		// Should be empty or contain valid user IDs (depending on other test data)
		// The important thing is that it doesn't fail due to incomplete profile
		t.Logf("User has %d recommendations", len(recResp.Recommendations))
	})
}
