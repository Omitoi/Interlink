package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

// UserIDKey is the key type for storing user ID in context
type UserIDKey string

// UserIDKey constant for context
const UserIDKeyValue UserIDKey = "userID"

// For backward compatibility and local usage
const userIDKey = UserIDKeyValue

func registerHandler(db *sql.DB) http.HandlerFunc {
	repo := NewAuthRepository(db)
	svc := NewAuthService(repo)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}

		type RegisterRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		tokenString, newID, err := svc.Register(r.Context(), req.Email, req.Password)
		if err != nil {
			if err.Error() == "missing_fields" {
				writeError(w, http.StatusBadRequest, "missing_fields")
				return
			}
			if errors.Is(err, ErrEmailExists) {
				writeError(w, http.StatusConflict, "email_exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "register_error")
			log.Println("Error registering user:", err)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{"token": tokenString, "id": newID})
	}
}

func loginHandler(db *sql.DB) http.HandlerFunc {
	repo := NewAuthRepository(db)
	svc := NewAuthService(repo)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}

		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}

		tokenString, userID, err := svc.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			if err.Error() == "missing_fields" {
				writeError(w, http.StatusBadRequest, "missing_fields")
				return
			}
			if errors.Is(err, ErrInvalidCredentials) {
				writeError(w, http.StatusUnauthorized, "invalid_credentials")
				return
			}
			writeError(w, http.StatusInternalServerError, "login_error")
			log.Println("Error logging in:", err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"token": tokenString, "id": userID})
	}
}

func authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		repo := NewAuthRepository(db)
		svc := NewAuthService(repo)

		userID, err := svc.ValidateToken(tokenStr)
		if err != nil {
			if err.Error() == "invalid_token_claims" || err.Error() == "invalid_user_id_in_token" || err.Error() == "invalid_token" {
				writeError(w, http.StatusUnauthorized, err.Error())
				return
			}
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		// Update last_online
		err = svc.UpdateLastOnline(r.Context(), userID)
		if err != nil {
			log.Println("Failed to update last_online:", err)
		}

		next(w, r.WithContext(context.WithValue(r.Context(), userIDKey, userID)))
	}
}
