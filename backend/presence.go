package main

import (
  "database/sql"
  "net/http"
)

func mePingHandler(db *sql.DB) http.HandlerFunc {
  return func(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
      http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed); return
    }
    userID, ok := getUserIDFromBearer(r)
    if !ok { http.Error(w, "Unauthorized", http.StatusUnauthorized); return }

    _, _ = db.Exec(`UPDATE users SET last_online = NOW() WHERE id = $1`, userID)
    w.WriteHeader(http.StatusNoContent)
  }
}

func isOnlineNow(db *sql.DB, userID int) (bool, error) {
	var online bool
	err := db.QueryRow(`
		SELECT COALESCE(last_online > NOW() - INTERVAL '90 seconds', FALSE) AS online
        FROM users
        WHERE id = $1
	`, userID).Scan(&online)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return online, err
}