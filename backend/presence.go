package main

import (
	"context"
	"database/sql"
	"net/http"
)

func mePingHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		userID := r.Context().Value(userIDKey).(int)

		_, _ = db.Exec(`UPDATE users SET last_online = NOW() WHERE id = $1`, userID)
		w.WriteHeader(http.StatusNoContent)
	})
}

func isOnlineNow(ctx context.Context, db *sql.DB, userID int) (bool, error) {
	var online bool
	err := db.QueryRowContext(ctx, `
		SELECT COALESCE(last_online > NOW() - INTERVAL '90 seconds', FALSE) AS online
        FROM users
        WHERE id = $1
	`, userID).Scan(&online)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return online, err
}
