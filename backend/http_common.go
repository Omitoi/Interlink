package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// --- Response helpers ---
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func jsonRawOrArray(raw json.RawMessage) interface{} {
	if len(raw) == 0 {
		return []interface{}{}
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return []interface{}{}
	}
	return v
}

func fetchBasicUserInfo(db *sql.DB, userID int) (displayName, profilePicture string, err error) {
	err = db.QueryRow(`
        SELECT
            COALESCE(p.display_name, 'User ' || u.id::text) AS display_name,
            COALESCE(p.profile_picture_file, 'avatar_placeholder.png') AS profile_picture_file
        FROM users u
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE u.id = $1
    `, userID).Scan(&displayName, &profilePicture)
	return
}

func fetchProfileInfo(db *sql.DB, userID int) (aboutMe, displayName, profilePicture string, err error) {
	var profilePictureSQL sql.NullString
	err = db.QueryRow(
		"SELECT about_me, display_name, profile_picture_file FROM profiles WHERE user_id = $1",
		userID,
	).Scan(&aboutMe, &displayName, &profilePictureSQL)

	if profilePictureSQL.Valid && strings.TrimSpace(profilePictureSQL.String) != "" {
		profilePicture = profilePictureSQL.String
	} else {
		profilePicture = "avatar_placeholder.png"
	}
	return
}

// withTx wraps a function in a database transaction.
// - Ensures COMMIT on success, ROLLBACK on errors or panics.
// - Keeps handler bodies tiny and all state changes atomic.
func withTx(ctx context.Context, db *sql.DB, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}

	defer func() {
		// If the callback panics, make sure to rollback before re-panicking
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// ConnectionRow represents a connection between two users
type ConnectionRow struct {
	ID           int
	UserID       int // requester
	TargetUserID int // addressee
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// loadPairForUpdate returns the *latest* connection row between two users
// (in EITHER direction), and takes a row lock (`FOR UPDATE`) so no other
// concurrent request can modify it until our transaction finishes.
//   - Returns (nil, nil) if no row exists yet.
//   - We ORDER BY updated_at DESC, id DESC to prefer the most recent row
//     in case historical/duplicate rows exist from legacy behavior.
func loadPairForUpdate(tx *sql.Tx, a, b int) (*ConnectionRow, error) {

	row := tx.QueryRow(`
		SELECT id, user_id, target_user_id, status, created_at, updated_at
		FROM connections
		WHERE (user_id = $1 AND target_user_id = $2)
		   OR (user_id = $2 AND target_user_id = $1)
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, a, b)

	var c ConnectionRow
	if err := row.Scan(&c.ID, &c.UserID, &c.TargetUserID, &c.Status, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			log.Println("SQL error: no rows")
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}
