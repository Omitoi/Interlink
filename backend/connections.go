package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func connectionsHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		rows, err := db.Query(`
            SELECT 
                CASE 
                    WHEN user_id = $1 THEN target_user_id
                    ELSE user_id
                END AS connection_id
            FROM connections
            WHERE (user_id = $1 OR target_user_id = $1) AND status = 'accepted'
        `, userID)
		if err != nil {
			http.Error(w, "Error fetching connections", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var connections []int
		for rows.Next() {
			var connID int
			if err := rows.Scan(&connID); err == nil {
				connections = append(connections, connID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string][]int{"connections": connections})
	})
}

// Handler function that returns all the user ids with which the user has pending connection requests
// Used for listing all the users that have requested connection
func requestsHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {

		// Make sure the method is correct
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}

		userID := r.Context().Value(userIDKey).(int)

		rows, err := db.Query(`
			SELECT user_id AS peer_user_id
			FROM connections
			WHERE target_user_id = $1 AND status = 'pending'
			ORDER BY created_at DESC, id DESC
		`, userID)
		if err != nil {
			http.Error(w, "Error fetching pending requests", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var requests []int
		for rows.Next() {
			var peerID int
			if err := rows.Scan(&peerID); err == nil {
				requests = append(requests, peerID)
			}
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
// Creates a pending request from the authenticated user to {id}.
// If the opposite side had already requested, we auto-accept.
func requestConnectionHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		// 1) Method and path parsing
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// Expect: /connections/{id}/request
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
		if targetID == me {
			writeError(w, http.StatusBadRequest, "invalid_target")
			return
		}

		// Ensure target exists & is viewable/recommendable.
		var exists bool
		if err := db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM users u
				JOIN profiles p ON p.user_id = u.id
				WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
			)
		`, targetID).Scan(&exists); err != nil || !exists {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		// Compute the "currently recommendable" boolean up-front.
		// We *won't* enforce this yet; first we check for an opposite pending inside the tx.
		isRec, err := isCurrentlyRecommendable(db, me, targetID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("isCurrentlyRecommendable error:", err)
			return
		}

		// 2) Do the state change inside a transaction to avoid races
		type response struct {
			State        string `json:"state"`
			ConnectionID *int   `json:"connection_id,omitempty"`
		}
		var resp response
		wroteErr := false

		err = withTx(r.Context(), db, func(tx *sql.Tx) error {
			// Lock existing connection row between me and target (either direction)
			row, err := loadPairForUpdate(tx, me, targetID)
			if err != nil {
				return err
			}

			// 1) Mutual-request rule: if THEY already requested ME and it's pending,
			//    we auto-accept regardless of current recommendations.
			if row != nil && row.Status == "pending" && row.UserID == targetID && row.TargetUserID == me {
				if err := tx.QueryRow(`
					UPDATE connections SET status = 'accepted' 
					WHERE id = $1 RETURNING id`, row.ID,
				).Scan(&resp.ConnectionID); err != nil {
					return err
				}
				resp.State = "accepted"
				return nil
			}

			// 2) Handle existing connection states before enforcing recommendation policy
			if row != nil {
				// There is already a row (some state) between us.
				switch row.Status {
				case "pending":
					resp.State = "pending"
					resp.ConnectionID = &row.ID
					return nil

				case "accepted":
					// Already connected -> idempotent OK
					resp.State = "accepted"
					resp.ConnectionID = &row.ID
					return nil
				case "dismissed", "disconnected":
					writeError(w, http.StatusConflict, "invalid_state")
					wroteErr = true
					return nil
				default:
					// Unknown enum value
					writeError(w, http.StatusConflict, "invalid_state")
					wroteErr = true
					return nil
				}
			}

			// 3) No existing row - enforce recommendation policy for new connections
			if !isRec {
				writeError(w, http.StatusNotFound, "not_found")
				wroteErr = true
				return nil
			}

			// 4) Create new pending request
			if err := tx.QueryRow(`
				INSERT INTO connections (user_id, target_user_id, status)
				VALUES ($1, $2, 'pending')
				RETURNING id
			`, me, targetID).Scan(&resp.ConnectionID); err != nil {
				return err
			}
			resp.State = "pending"
			return nil
		})

		if err != nil {
			// Database or transaction error
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("requestConnectionHandler tx error:", err)
			return
		}
		if wroteErr {
			return // error already written inside the tx
		}
		if resp.State == "" {
			writeError(w, http.StatusInternalServerError, "unknown_state")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// POST /connections/{id}/accept
// Accepts a pending connection request from {id} -> me.
// - If the opposite direction is pending (me -> {id}), there's nothing to accept -> 404.
// - If already accepted, idempotent OK.
// - Terminal/other states -> 409 invalid_state.
func acceptConnectionHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		// 1) Method + path shape: /connections/{id}/accept
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
		if targetID == me {
			writeError(w, http.StatusBadRequest, "invalid_target")
			return
		}

		// Ensure target exists & profile complete.
		var exists bool
		if err := db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM users u
				JOIN profiles p ON p.user_id = u.id
				WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
			)
		`, targetID).Scan(&exists); err != nil || !exists {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		type response struct {
			State        string `json:"state"`
			ConnectionID *int   `json:"connection_id,omitempty"`
		}
		var resp response
		wroteErr := false

		// 2) Do state change atomically
		err = withTx(r.Context(), db, func(tx *sql.Tx) error {
			row, err := loadPairForUpdate(tx, me, targetID)
			if err != nil {
				return err
			}
			if row == nil {
				// No relationship row at all -> nothing to accept
				writeError(w, http.StatusNotFound, "not_found")
				wroteErr = true
				return nil
			}

			switch row.Status {
			case "pending":
				// Accept only if THEY requested ME (targetID -> me)
				if row.UserID == targetID && row.TargetUserID == me {
					// Return id of the now-accepted row
					if err := tx.QueryRow(`
					UPDATE connections
					SET status = 'accepted'
					WHERE id = $1
					RETURNING id
				`, row.ID).Scan(&resp.ConnectionID); err != nil {
						return err
					}
					resp.State = "accepted"
					return nil
				}
				// It's my own pending (me -> targetID): nothing to accept
				writeError(w, http.StatusNotFound, "not_found")
				wroteErr = true
				return nil

			case "accepted":
				// Idempotent: already accepted
				resp.State = "accepted"
				resp.ConnectionID = &row.ID
				return nil

			case "dismissed", "disconnected":
				// Not a valid transition to accept
				writeError(w, http.StatusConflict, "invalid_state")
				wroteErr = true
				return nil

			default:
				writeError(w, http.StatusConflict, "invalid_state")
				wroteErr = true
				return nil
			}
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("acceptConnectionHandler tx error:", err)
			return
		}
		if wroteErr {
			return // already wrote the error inside tx
		}
		if resp.State == "" {
			writeError(w, http.StatusInternalServerError, "unknown_state")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// POST /connections/{id}/decline
// Declines a pending connection request from {id} -> me by setting status to 'dismissed'.
// - If the pending is me -> {id}, there's nothing to decline -> 404.
// - If already dismissed, idempotent OK (returns {state:"dismissed"}).
// - If accepted or disconnected, invalid transition -> 409.
func declineConnectionHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		log.Println("A connection was declined.")
		// 1) Method + path: /connections/{id}/decline
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
		if targetID == me {
			log.Println("The target ID and the requester's ID were the same")
			writeError(w, http.StatusBadRequest, "invalid_target")
			return
		}

		// Ensure target exists & profile complete.
		var exists bool
		if err := db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM users u
				JOIN profiles p ON p.user_id = u.id
				WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
			)
		`, targetID).Scan(&exists); err != nil || !exists {
			log.Println("Error accessing database: ", err)
			if !exists {
				log.Println(targetID, " does not exist.")
			}
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		type response struct {
			State string `json:"state"`
		}
		var resp response

		// 2) Do the decision atomically
		err = withTx(r.Context(), db, func(tx *sql.Tx) error {
			row, err := loadPairForUpdate(tx, me, targetID)
			if err != nil {
				return err
			}
			if row == nil {
				// No relationship row at all -> nothing to decline
				log.Println("No relationship row at all")
				writeError(w, http.StatusNotFound, "not_found")
				return nil
			}

			switch row.Status {
			case "pending":
				// Valid decline only if THEY requested ME (targetID -> me).
				if row.UserID == targetID && row.TargetUserID == me {
					if _, err := tx.Exec(`UPDATE connections SET status = 'dismissed' WHERE id = $1`, row.ID); err != nil {
						return err
					}
					resp.State = "dismissed"
					return nil
				}
				// It's my own pending (me -> targetID): nothing to decline.
				log.Println("Connection set pending by the user. Nothing to decline.")
				writeError(w, http.StatusNotFound, "not_found")
				return nil

			case "dismissed":
				// Idempotent OK: already declined/canceled earlier.
				resp.State = "dismissed"
				return nil

			case "accepted", "disconnected":
				// Not a valid transition to decline.
				log.Println("Not a valid transition to decline.")
				writeError(w, http.StatusConflict, "invalid_state")
				return nil

			default:
				writeError(w, http.StatusConflict, "invalid_state")
				log.Println("The connection status was unexpected")
				return nil
			}
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("declineConnectionHandler tx error:", err)
			return
		}
		if resp.State != "" {
			writeJSON(w, http.StatusOK, resp)
		}
	})
}

// POST /connections/{id}/cancel
// Cancels *my own* pending connection request (me -> {id}) by setting status to 'dismissed'.
// - If the pending is {id} -> me, there's nothing to cancel -> 404 (use decline instead).
// - If already dismissed, idempotent OK (returns {state:"dismissed"}).
// - If accepted or disconnected, invalid transition -> 409.
func cancelConnectionRequestHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		// 1) Method + path: /connections/{id}/cancel
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
		if targetID == me {
			writeError(w, http.StatusBadRequest, "invalid_target")
			return
		}

		// Ensure target exists & profile complete.
		var exists bool
		if err := db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM users u
				JOIN profiles p ON p.user_id = u.id
				WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
			)
		`, targetID).Scan(&exists); err != nil || !exists {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		type response struct {
			State string `json:"state"`
		}
		var resp response

		// 2) Do the change atomically
		err = withTx(r.Context(), db, func(tx *sql.Tx) error {
			row, err := loadPairForUpdate(tx, me, targetID)
			if err != nil {
				return err
			}
			if row == nil {
				// No relationship row at all -> nothing to cancel
				writeError(w, http.StatusNotFound, "not_found")
				return nil
			}

			switch row.Status {
			case "pending":
				// Valid cancel only if I am the requester (me -> targetID).
				if row.UserID == me && row.TargetUserID == targetID {
					if _, err := tx.Exec(`UPDATE connections SET status = 'dismissed' WHERE id = $1`, row.ID); err != nil {
						return err
					}
					resp.State = "dismissed"
					return nil
				}
				// It's their pending (targetID -> me): nothing to cancel.
				writeError(w, http.StatusNotFound, "not_found")
				return nil

			case "dismissed":
				// Idempotent OK: already canceled/declined earlier.
				resp.State = "dismissed"
				return nil

			case "accepted", "disconnected":
				// Can't cancel a request that isn't pending (use disconnect for accepted).
				writeError(w, http.StatusConflict, "invalid_state")
				return nil

			default:
				writeError(w, http.StatusConflict, "invalid_state")
				return nil
			}
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("cancelConnectionRequestHandler tx error:", err)
			return
		}
		if resp.State != "" {
			writeJSON(w, http.StatusOK, resp)
		}
	})
}

// DELETE /connections/{id}
// Disconnects two users by changing status from 'accepted' to 'disconnected'.
// - Either party can call it.
// - If already disconnected, idempotent 204.
// - If pending or dismissed, invalid transition -> 409.
// - If no relationship row exists, 404.
func disconnectConnectionHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		// 1) Method + path
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// Expect: /connections/{id}
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
		if targetID == me {
			writeError(w, http.StatusBadRequest, "invalid_target")
			return
		}

		// Ensure target exists & profile complete.
		var exists bool
		if err := db.QueryRow(`
			SELECT EXISTS (
				SELECT 1
				FROM users u
				JOIN profiles p ON p.user_id = u.id
				WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
			)
		`, targetID).Scan(&exists); err != nil || !exists {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		// We'll respond with 204 if we successfully disconnect OR if it was already disconnected.
		okNoContent := false

		// 2) Atomic state change
		err = withTx(r.Context(), db, func(tx *sql.Tx) error {
			row, err := loadPairForUpdate(tx, me, targetID)
			if err != nil {
				return err
			}
			if row == nil {
				// No relationship row at all
				writeError(w, http.StatusNotFound, "not_found")
				return nil
			}

			switch row.Status {
			case "accepted":
				// Valid: flip to disconnected
				if _, err := tx.Exec(`UPDATE connections SET status = 'disconnected' WHERE id = $1`, row.ID); err != nil {
					return err
				}
				okNoContent = true
				return nil

			case "disconnected":
				// Idempotent: already disconnected
				okNoContent = true
				return nil

			case "pending", "dismissed":
				// Can't disconnect something that's not an accepted connection
				writeError(w, http.StatusConflict, "invalid_state")
				return nil

			default:
				writeError(w, http.StatusConflict, "invalid_state")
				return nil
			}
		})

		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			log.Println("disconnectConnectionHandler tx error:", err)
			return
		}

		if okNoContent {
			// 204 No Content is idiomatic for successful DELETE (and idempotent re-DELETE).
			w.WriteHeader(http.StatusNoContent)
		}
	})
}
