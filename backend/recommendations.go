package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
)

func recommendationsHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		// Gate by profile completion
		var isComplete bool
		err := db.QueryRow("SELECT COALESCE(is_complete, FALSE) FROM profiles WHERE user_id = $1", userID).Scan(&isComplete)
		if err == sql.ErrNoRows || !isComplete {
			writeError(w, http.StatusForbidden, "incomplete_profile")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}
		recommendations, err := getRecommendedUserIDs(db, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "recommendation_error")
			return
		}
		// Filter out dismissed recommendations
		if len(recommendations) > 0 {
			rows, derr := db.Query(`SELECT dismissed_user_id FROM dismissed_recommendations WHERE user_id = $1`, userID)
			if derr == nil {
				defer rows.Close()
				dismissed := make(map[int]struct{})
				for rows.Next() {
					var d int
					if rows.Scan(&d) == nil {
						dismissed[d] = struct{}{}
					}
				}
				filtered := make([]int, 0, len(recommendations))
				for _, id := range recommendations {
					if _, gone := dismissed[id]; !gone {
						filtered = append(filtered, id)
					}
				}
				recommendations = filtered
			}
		}
		writeJSON(w, http.StatusOK, map[string][]int{"recommendations": recommendations})
	})
}

// GET /recommendations/detailed - Returns recommendations with scores
func recommendationsDetailedHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		// Gate by profile completion
		var isComplete bool
		err := db.QueryRow("SELECT COALESCE(is_complete, FALSE) FROM profiles WHERE user_id = $1", userID).Scan(&isComplete)
		if err == sql.ErrNoRows || !isComplete {
			writeError(w, http.StatusForbidden, "incomplete_profile")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}

		results, err := getRecommendationsWithScores(db, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "recommendation_error")
			return
		}

		// Filter out dismissed recommendations
		if len(results) > 0 {
			rows, derr := db.Query(`SELECT dismissed_user_id FROM dismissed_recommendations WHERE user_id = $1`, userID)
			if derr == nil {
				defer rows.Close()
				dismissed := make(map[int]struct{})
				for rows.Next() {
					var d int
					if rows.Scan(&d) == nil {
						dismissed[d] = struct{}{}
					}
				}
				filtered := make([]RecommendationResult, 0, len(results))
				for _, result := range results {
					if _, gone := dismissed[result.UserID]; !gone {
						filtered = append(filtered, result)
					}
				}
				results = filtered
			}
		}

		writeJSON(w, http.StatusOK, map[string][]RecommendationResult{"recommendations": results})
	})
}

// POST /recommendations/{id}/dismiss
func dismissRecommendationHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) != 3 || parts[0] != "recommendations" || parts[2] != "dismiss" {
			http.NotFound(w, r)
			return
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		userID := r.Context().Value(userIDKey).(int)
		// Ensure target user exists & is currently recommended (optional stronger check)
		var exists bool
		err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM users JOIN profiles ON users.id = profiles.user_id WHERE users.id = $1 AND profiles.is_complete = TRUE)", id).Scan(&exists)
		if err != nil || !exists || id == userID {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		// Insert dismissal (ignore duplicates)
		_, err = db.Exec(`INSERT INTO dismissed_recommendations (user_id, dismissed_user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "dismiss_error")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]bool{"dismissed": true})
	})
}

// isCurrentlyRecommendable returns true if targetID is in the *current*
// recommendations for `me`, after subtracting any users the caller has dismissed.
// This mirrors the /recommendations filtering so the policy is consistent.
func isCurrentlyRecommendable(db *sql.DB, me, targetID int) (bool, error) {
	recs, err := getRecommendedUserIDs(db, me)
	if err != nil {
		return false, err
	}
	// Build a set for O(1) membership checks
	recSet := make(map[int]struct{}, len(recs))
	for _, id := range recs {
		recSet[id] = struct{}{}
	}
	// Remove dismissed
	rows, derr := db.Query(`SELECT dismissed_user_id FROM dismissed_recommendations WHERE user_id = $1`, me)
	if derr == nil {
		defer rows.Close()
		for rows.Next() {
			var d int
			if rows.Scan(&d) == nil {
				delete(recSet, d)
			}
		}
	}
	_, ok := recSet[targetID]
	return ok, nil
}
