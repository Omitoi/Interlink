package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// Dispatcher for /users/* to route summary/profile/bio
func usersDispatcher(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 || parts[0] != "users" {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 2 {
			userHandler(db).ServeHTTP(w, r)
			return
		}
		if len(parts) == 3 {
			switch parts[2] {
			case "profile":
				userProfileHandler(db).ServeHTTP(w, r)
			case "bio":
				userBioHandler(db).ServeHTTP(w, r)
			default:
				http.NotFound(w, r)
			}
			return
		}
		http.NotFound(w, r)
	}
}

func userHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "users" {
			http.NotFound(w, r)
			return
		}
		userID, err := strconv.Atoi(parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}

		resp, err := svc.GetBasicUserInfoWithPresence(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// GET /users/{id}/profile
func userProfileHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "users" || parts[2] != "profile" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		requesterID := r.Context().Value(userIDKey).(int)

		resp, err := svc.GetTargetProfile(r.Context(), requesterID, targetID)
		if err != nil {
			if err == ErrNotFound {
				writeError(w, http.StatusNotFound, "not_found")
			} else {
				writeError(w, http.StatusInternalServerError, "db_error")
			}
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

func userBioHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "users" || parts[2] != "bio" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		requesterID := r.Context().Value(userIDKey).(int)

		resp, err := svc.GetTargetBio(r.Context(), requesterID, targetID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

func completeProfileHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPatch {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}

		var req ProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		userID := r.Context().Value(userIDKey).(int)

		err := svc.UpsertProfile(r.Context(), userID, req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "profile_save_error")
			log.Println("Error saving profile:", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func meHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		resp, err := svc.GetMeBasicProfile(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// --- Self profile view handler (GET /me/profile) returning profile details ---
func meProfileHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		userID := r.Context().Value(userIDKey).(int)

		response, err := svc.GetMeFullProfile(r.Context(), userID)
		if err != nil {
			if err == ErrNotFound {
				writeError(w, http.StatusNotFound, "profile_not_found")
			} else {
				writeError(w, http.StatusInternalServerError, "database_error")
			}
			return
		}

		writeJSON(w, http.StatusOK, response)
	})
}

// Bio handlers (GET /me/bio and GET /users/{id}/bio) - simplified placeholder extraction
func meBioHandler(db *sql.DB) http.HandlerFunc {
	repo := NewUserProfileRepository(db)
	svc := NewUserProfileService(repo, db)

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		resp, err := svc.GetMeBio(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// Backward compatibility wrapper for avatar.go
func canViewUser(ctx context.Context, db *sql.DB, viewerID, targetID int) bool {
	repo := NewUserProfileRepository(db)
	return repo.CanViewUser(ctx, viewerID, targetID)
}
