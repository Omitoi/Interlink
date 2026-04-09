package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Handler function that returns all the user ids with which the user has accepted connections
// Used for listing all the users that the user has accepted connections with
func connectionsHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		connections, err := svc.GetConnections(r.Context(), userID)
		if err != nil {
			http.Error(w, "Error fetching connections", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]int{"connections": connections})
	})
}

// Handler function that returns all the user ids with which the user has pending connection requests
// Used for listing all the users that have requested connection
func requestsHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {

		// Make sure the method is correct
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}

		userID := r.Context().Value(userIDKey).(int)

		requests, err := svc.GetRequests(r.Context(), userID)
		if err != nil {
			http.Error(w, "Error fetching pending requests", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]int{"requests": requests})
	})
}

// Handler functions for creating and altering the connection status between users
//
// TERMINOLOGY
// request: create pending (or auto-accept if opposite pending exists).
// accept: pending → accepted.
// decline (by addressee): pending → dismissed.
// cancel (by requester): pending → dismissed (same terminal state, different actor).
// disconnect (either party): accepted → disconnected

// A dispatcher router function for all /connections/{id}/... requests
func connectionsActionsRouter(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[0] != "connections" {
			http.NotFound(w, r)
			return
		}

		// DELETE /connections/{id} → disconnect
		if r.Method == http.MethodDelete && len(parts) == 2 {
			disconnectConnectionHandler(db).ServeHTTP(w, r)
			return
		}

		// POST /connections/{id}/(request|accept|decline|cancel)
		if r.Method == http.MethodPost && len(parts) == 3 {
			switch parts[2] {
			case "request":
				requestConnectionHandler(db).ServeHTTP(w, r)
			case "accept":
				acceptConnectionHandler(db).ServeHTTP(w, r)
			case "decline":
				declineConnectionHandler(db).ServeHTTP(w, r)
			case "cancel":
				cancelConnectionRequestHandler(db).ServeHTTP(w, r)
			default:
				http.NotFound(w, r)
			}
			return
		}

		// Anything else under /connections/ → 404
		http.NotFound(w, r)
	}
}

// POST /connections/{id}/request
func requestConnectionHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "connections" || parts[2] != "request" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		me := r.Context().Value(userIDKey).(int)

		state, connID, err := svc.RequestConnection(r.Context(), me, targetID)
		if err != nil {
			if errors.Is(err, ErrInvalidTarget) {
				writeError(w, http.StatusBadRequest, "invalid_target")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				writeError(w, http.StatusConflict, "invalid_state")
				return
			}
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("requestConnectionHandler error:", err)
			return
		}

		type response struct {
			State        string `json:"state"`
			ConnectionID *int   `json:"connection_id,omitempty"`
		}
		writeJSON(w, http.StatusOK, response{State: state, ConnectionID: connID})
	})
}

// POST /connections/{id}/accept
func acceptConnectionHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "connections" || parts[2] != "accept" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		me := r.Context().Value(userIDKey).(int)

		state, connID, err := svc.AcceptConnection(r.Context(), me, targetID)
		if err != nil {
			if errors.Is(err, ErrInvalidTarget) {
				writeError(w, http.StatusBadRequest, "invalid_target")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				writeError(w, http.StatusConflict, "invalid_state")
				return
			}
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("acceptConnectionHandler error:", err)
			return
		}

		type response struct {
			State        string `json:"state"`
			ConnectionID *int   `json:"connection_id,omitempty"`
		}
		writeJSON(w, http.StatusOK, response{State: state, ConnectionID: connID})
	})
}

// POST /connections/{id}/decline
func declineConnectionHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		log.Println("A connection was declined.")
		if r.Method != http.MethodPost {
			log.Println("Invalid method.")
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "connections" || parts[2] != "decline" {
			log.Println("Invalid URL")
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Println("Invalid format for user ID")
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		me := r.Context().Value(userIDKey).(int)

		state, err := svc.DeclineConnection(r.Context(), me, targetID)
		if err != nil {
			if errors.Is(err, ErrInvalidTarget) {
				log.Println("The target ID and the requester's ID were the same")
				writeError(w, http.StatusBadRequest, "invalid_target")
				return
			}
			if errors.Is(err, ErrNotFound) {
				log.Println("Error accessing database or not found")
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				log.Println("Not a valid transition")
				writeError(w, http.StatusConflict, "invalid_state")
				return
			}
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("declineConnectionHandler error:", err)
			return
		}

		type response struct {
			State string `json:"state"`
		}
		if state != "" {
			writeJSON(w, http.StatusOK, response{State: state})
		}
	})
}

// POST /connections/{id}/cancel
func cancelConnectionRequestHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "connections" || parts[2] != "cancel" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		me := r.Context().Value(userIDKey).(int)

		state, err := svc.CancelConnection(r.Context(), me, targetID)
		if err != nil {
			if errors.Is(err, ErrInvalidTarget) {
				writeError(w, http.StatusBadRequest, "invalid_target")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				writeError(w, http.StatusConflict, "invalid_state")
				return
			}
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("cancelConnectionRequestHandler error:", err)
			return
		}

		type response struct {
			State string `json:"state"`
		}
		if state != "" {
			writeJSON(w, http.StatusOK, response{State: state})
		}
	})
}

// DELETE /connections/{id}
func disconnectConnectionHandler(db *sql.DB) http.HandlerFunc {
	repo := NewConnectionRepository()
	svc := NewConnectionService(db, repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != "connections" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		me := r.Context().Value(userIDKey).(int)

		okNoContent, err := svc.DisconnectConnection(r.Context(), me, targetID)
		if err != nil {
			if errors.Is(err, ErrInvalidTarget) {
				writeError(w, http.StatusBadRequest, "invalid_target")
				return
			}
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			if errors.Is(err, ErrInvalidState) {
				writeError(w, http.StatusConflict, "invalid_state")
				return
			}
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("disconnectConnectionHandler error:", err)
			return
		}

		if okNoContent {
			w.WriteHeader(http.StatusNoContent)
		}
	})
}

func targetExistsAndComplete(ctx context.Context, db *sql.DB, targetID int) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM users u
				JOIN profiles p ON p.user_id = u.id
				WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
			)
		`, targetID).Scan(&exists)
	return exists, err
}
