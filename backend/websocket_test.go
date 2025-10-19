package main

import (
	"net/http/httptest"
	"testing"
)

func TestGetUserIDFromRequest(t *testing.T) {
	// Initialize JWT secret for testing
	jwtSecret = []byte("test-secret-key-for-testing")

	// Create a test user to get a valid token
	user := createTestUser(t, "websocket_test@example.com", "password123")

	t.Run("Valid Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)

		userID, ok := getUserIDFromRequest(req)
		if !ok {
			t.Error("Expected getUserIDFromRequest to succeed")
		}
		if userID != user.ID {
			t.Errorf("Expected userID %d, got %d", user.ID, userID)
		}
	})

	t.Run("Valid token query parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?token="+user.Token, nil)

		userID, ok := getUserIDFromRequest(req)
		if !ok {
			t.Error("Expected getUserIDFromRequest to succeed with query param")
		}
		if userID != user.ID {
			t.Errorf("Expected userID %d, got %d", user.ID, userID)
		}
	})

	t.Run("No authentication", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)

		userID, ok := getUserIDFromRequest(req)
		if ok {
			t.Error("Expected getUserIDFromRequest to fail")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Invalid Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer invalid_token")

		userID, ok := getUserIDFromRequest(req)
		if ok {
			t.Error("Expected getUserIDFromRequest to fail with invalid token")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Invalid token query parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test?token=invalid_token", nil)

		userID, ok := getUserIDFromRequest(req)
		if ok {
			t.Error("Expected getUserIDFromRequest to fail with invalid query token")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Malformed Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "NotBearer "+user.Token)

		userID, ok := getUserIDFromRequest(req)
		if ok {
			t.Error("Expected getUserIDFromRequest to fail with malformed header")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Short Authorization header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bear")

		userID, ok := getUserIDFromRequest(req)
		if ok {
			t.Error("Expected getUserIDFromRequest to fail with short header")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Header takes precedence over query param", func(t *testing.T) {
		user2 := createTestUser(t, "websocket_test2@example.com", "password123")

		req := httptest.NewRequest("GET", "/test?token="+user2.Token, nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)

		userID, ok := getUserIDFromRequest(req)
		if !ok {
			t.Error("Expected getUserIDFromRequest to succeed")
		}
		if userID != user.ID {
			t.Errorf("Expected userID from header %d, got %d", user.ID, userID)
		}
	})
}

func TestParseUserIDFromJWT(t *testing.T) {
	// Initialize JWT secret for testing
	jwtSecret = []byte("test-secret-key-for-testing")

	// Create a test user to get a valid token
	user := createTestUser(t, "jwt_test@example.com", "password123")

	t.Run("Valid JWT token", func(t *testing.T) {
		userID, ok := parseUserIDFromJWT(user.Token)
		if !ok {
			t.Error("Expected parseUserIDFromJWT to succeed")
		}
		if userID != user.ID {
			t.Errorf("Expected userID %d, got %d", user.ID, userID)
		}
	})

	t.Run("Invalid JWT token", func(t *testing.T) {
		userID, ok := parseUserIDFromJWT("invalid.jwt.token")
		if ok {
			t.Error("Expected parseUserIDFromJWT to fail")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Empty token", func(t *testing.T) {
		userID, ok := parseUserIDFromJWT("")
		if ok {
			t.Error("Expected parseUserIDFromJWT to fail with empty token")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})

	t.Run("Malformed JWT", func(t *testing.T) {
		userID, ok := parseUserIDFromJWT("not.a.jwt")
		if ok {
			t.Error("Expected parseUserIDFromJWT to fail with malformed JWT")
		}
		if userID != 0 {
			t.Errorf("Expected userID 0, got %d", userID)
		}
	})
}

func TestJwtParse(t *testing.T) {
	// Initialize JWT secret for testing
	jwtSecret = []byte("test-secret-key-for-testing")

	// Create a test user to get a valid token
	user := createTestUser(t, "jwt_parse_test@example.com", "password123")

	t.Run("Valid JWT token", func(t *testing.T) {
		token, err := jwtParse(user.Token)
		if err != nil {
			t.Errorf("Expected jwtParse to succeed, got error: %v", err)
		}
		if token == nil {
			t.Error("Expected non-nil token")
			return
		}
		if !token.Valid {
			t.Error("Expected token to be valid")
		}
	})

	t.Run("Invalid JWT token", func(t *testing.T) {
		token, err := jwtParse("invalid.jwt.token")
		if err == nil {
			t.Error("Expected jwtParse to fail")
		}
		if token != nil && token.Valid {
			t.Error("Expected token to be invalid")
		}
	})

	t.Run("Empty token", func(t *testing.T) {
		token, err := jwtParse("")
		if err == nil {
			t.Error("Expected jwtParse to fail with empty token")
		}
		if token != nil && token.Valid {
			t.Error("Expected token to be invalid")
		}
	})
}
