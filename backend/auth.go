package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"	
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

// UserIDKey is the key type for storing user ID in context
type UserIDKey string

// UserIDKey constant for context
const UserIDKeyValue UserIDKey = "userID"

// For backward compatibility and local usage
const userIDKey = UserIDKeyValue

func registerHandler(db *sql.DB) http.HandlerFunc {
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

		// Clean up input
		req.Email = strings.TrimSpace(req.Email)
		req.Password = strings.TrimSpace(req.Password)
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "missing_fields")
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash_error")
			log.Println("Error hashing password:", err)
			return
		}

		var newID int
		err = db.QueryRowContext(r.Context(),
			"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
			req.Email, string(hashedPassword),
		).Scan(&newID)
		if err != nil {
			var pqErr *pq.Error
			if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
				writeError(w, http.StatusConflict, "email_exists")
				return
			}
			writeError(w, http.StatusInternalServerError, "register_error")
			log.Println("Error saving user to database:", err)
			return
		}

		// Update last_online for the new user
		_, err = db.ExecContext(r.Context(), "UPDATE users SET last_online = NOW() WHERE id = $1", newID)
		if err != nil {
			log.Println("Failed to update last_online for new user:", err)
		}

		// Generate JWT token for automatic login
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": newID,
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token_generation_error")
			log.Println("Error generating token for new user:", err)
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{"token": tokenString, "id": newID})
	}
}

func loginHandler(db *sql.DB) http.HandlerFunc {
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

		req.Email = strings.TrimSpace(req.Email)
		req.Password = strings.TrimSpace(req.Password)
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "missing_fields")
			return
		}

		var userID int
		var passwordHash string
		err := db.QueryRowContext(r.Context(), "SELECT id, password_hash FROM users WHERE email = $1", req.Email).Scan(&userID, &passwordHash)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		} else if err != nil {
			log.Println("Error querying user:", err)
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}

		// Compare the provided password with the stored hash
		if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_credentials")
			return
		}

		// Update last_online
		_, err = db.ExecContext(r.Context(), "UPDATE users SET last_online = NOW() WHERE id = $1", userID)
		if err != nil {
			log.Println("Failed to update last_online:", err)
			// Don't fail login, just log the error
		}

		// Generate JWT token
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"user_id": userID,
			"exp":     time.Now().Add(24 * time.Hour).Unix(),
		})
		tokenString, err := token.SignedString(jwtSecret)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token_generation_error")
			log.Println("Error generating token:", err)
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
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			writeError(w, http.StatusUnauthorized, "invalid_token")
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_token_claims")
			return
		}
		userID, ok := claims["user_id"].(float64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_user_id_in_token")
			return
		}
		// Update last_online
		// Update last_online
		_, err = db.ExecContext(r.Context(), "UPDATE users SET last_online = NOW() WHERE id = $1", int(userID))
		if err != nil {
			log.Println("Failed to update last_online:", err)
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userIDKey, int(userID))))
	}
}
