package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func recommendationsHandler(db *sql.DB) http.HandlerFunc {
	repo := NewRecommendationRepository(db)
	svc := NewRecommendationService(repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)

		isComplete, err := svc.CheckProfileComplete(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}
		if !isComplete {
			writeError(w, http.StatusForbidden, "incomplete_profile")
			return
		}

		recommendations, err := svc.GetRecommendedUserIDs(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "recommendation_error")
			return
		}
		
		if recommendations == nil {
			recommendations = []int{}
		}

		writeJSON(w, http.StatusOK, map[string][]int{"recommendations": recommendations})
	})
}

// GET /recommendations/detailed - Returns recommendations with scores
func recommendationsDetailedHandler(db *sql.DB) http.HandlerFunc {
	repo := NewRecommendationRepository(db)
	svc := NewRecommendationService(repo)
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		
		isComplete, err := svc.CheckProfileComplete(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error")
			return
		}
		if !isComplete {
			writeError(w, http.StatusForbidden, "incomplete_profile")
			return
		}

		results, err := svc.GetRecommendationsWithScores(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "recommendation_error")
			return
		}

		if results == nil {
			results = []RecommendationResult{}
		}

		writeJSON(w, http.StatusOK, map[string][]RecommendationResult{"recommendations": results})
	})
}

// POST /recommendations/{id}/dismiss
func dismissRecommendationHandler(db *sql.DB) http.HandlerFunc {
	repo := NewRecommendationRepository(db)
	svc := NewRecommendationService(repo)
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
		
		err = svc.DismissRecommendation(r.Context(), userID, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found")
				return
			}
			writeError(w, http.StatusInternalServerError, "dismiss_error")
			return
		}
		
		writeJSON(w, http.StatusCreated, map[string]bool{"dismissed": true})
	})
}
