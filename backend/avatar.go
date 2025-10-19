package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const avatarRoot = "./uploads/avatars"

// POST /me/avatar  (multipart form, field name: "file")
// Or redirect to removeAvatar if method is DELETE
func myAvatarHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {

		me := r.Context().Value(userIDKey).(int)

		// Remove avatar if method is DELETE
		if r.Method == http.MethodDelete {
			if err := removeAvatar(db, me); err != nil {
				http.Error(w, "remove_failed", http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return

		}
		if r.Method != http.MethodPost {
			http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit to ~3MB
		r.Body = http.MaxBytesReader(w, r.Body, 3<<20)
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			http.Error(w, "file_too_large_or_missing", http.StatusRequestEntityTooLarge)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing_file", http.StatusBadRequest)
			return
		}
		defer f.Close()

		// Sniff MIME from the first bytes
		head := make([]byte, 512)
		n, _ := f.Read(head)
		ctype := http.DetectContentType(head[:n])
		if ctype != "image/jpeg" {
			http.Error(w, "only_jpeg_allowed", http.StatusBadRequest)
			return
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			http.Error(w, "seek_failed", http.StatusInternalServerError)
			return
		}

		// Make sure the directory exists
		if err := os.MkdirAll(avatarRoot, 0o755); err != nil {
			http.Error(w, "mkdir_failed", http.StatusInternalServerError)
			return
		}

		// Naming logic. For now we use userId.jpg.
		filename := fmt.Sprintf("%d.jpg", me)
		dst := filepath.Join(avatarRoot, filename)
		tmp := dst + ".tmp"

		out, err := os.Create(tmp)
		if err != nil {
			http.Error(w, "save_failed", http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(out, f); err != nil {
			out.Close()
			http.Error(w, "save_failed", http.StatusInternalServerError)
			return
		}
		out.Close()
		if err := os.Rename(tmp, dst); err != nil {
			http.Error(w, "save_failed", http.StatusInternalServerError)
			return
		}

		// Save the filename to database
		res, err := db.Exec(`
			UPDATE profiles 
			SET profile_picture_file = $1 WHERE user_id = $2
		`, filename, me)
		if err != nil {
			// If the database fails, leave the file but report the error.
			http.Error(w, "db_update_failed", http.StatusInternalServerError)
			return
		}
		aff, _ := res.RowsAffected()
		if aff == 0 {
			// The profile row has not been initialized yet.
			// Remove the file
			_ = os.Remove(dst)
			http.Error(w, "profile_not_initialized", http.StatusConflict)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	})
}

// GET /avatars/{id}
// Only show the requesting user OR a recommended/pending/accepted relationship
func getUserAvatarHandler(db *sql.DB) http.HandlerFunc {

	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)

			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// /avatars/{id}
		if len(parts) != 2 || parts[0] != "avatars" {
			http.NotFound(w, r)
			return
		}
		targetID, err := strconv.Atoi(parts[1])
		if err != nil {
			http.NotFound(w, r)

			return
		}

		me := r.Context().Value(userIDKey).(int)

		// Own picture ok. Otherwise a pending/accepted/recommended relationship must exist.
		if me != targetID {
			ok, _ := hasPendingOrAccepted(db, me, targetID)

			// Check if the requested user is recommendable and if so, allow viewing
			if !ok {
				ok, _ = isCurrentlyRecommendable(db, me, targetID)
			}
			if !ok {
				// 404 so that the file existence is not revealed to bad actors
				http.NotFound(w, r)
				return
			}
		}

		// Read the filename from the database
		filename, err := getProfilePictureFilename(db, targetID)
		var path string
		var contentType string

		if err != nil {
			// No custom profile picture filename in database, use placeholder
			filename = "avatar_placeholder.png"
			path = filepath.Join(avatarRoot, filename)
			contentType = "image/png"
		} else {
			// Check if the custom file actually exists
			path = filepath.Join(avatarRoot, filename)
			if _, err := os.Stat(path); err != nil {
				// Custom file doesn't exist, fall back to placeholder
				filename = "avatar_placeholder.png"
				path = filepath.Join(avatarRoot, filename)
				contentType = "image/png"
			} else {
				// Determine content type for custom file
				contentType = "image/jpeg"
				if strings.HasSuffix(filename, ".png") {
					contentType = "image/png"
				}
			}
		}

		// Final check: make sure the file we're about to serve exists
		if _, err := os.Stat(path); err != nil {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", contentType)
		// Light cache - busted in frontend ?ts=timestamp
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeFile(w, r, path)
	})
}

// Lightweight relationship check (pending/accepted)
func hasPendingOrAccepted(db *sql.DB, a, b int) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
            SELECT 1 FROM connections
            WHERE ((user_id=$1 AND target_user_id=$2) OR (user_id=$2 AND target_user_id=$1))
              AND status IN ('pending','accepted')
		)`, a, b).Scan(&exists)
	return exists, err
}

func getProfilePictureFilename(db *sql.DB, userID int) (string, error) {
	var fn sql.NullString
	err := db.QueryRow(`SELECT profile_picture_file FROM profiles WHERE user_id = $1`, userID).Scan(&fn)
	if err != nil {
		return "", err
	}
	if !fn.Valid || strings.TrimSpace(fn.String) == "" {
		return "", errors.New("no_picture")
	}
	return fn.String, nil
}

func removeAvatar(db *sql.DB, userID int) error {
	filename, err := getProfilePictureFilename(db, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No profile row; nothing to remove
			return nil
		}
		return fmt.Errorf("error reading current avatar filename: %w", err)
	}
	if filename != "" {
		// Protecting the path. Only using basename to avoid injection of ../ etc.
		fullPath := filepath.Join(avatarRoot, filepath.Base(filename))
		if rmErr := os.Remove(fullPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("error removing avatar file %q: %w", fullPath, rmErr)
		}
	}

	res, err := db.Exec(`UPDATE profiles SET profile_picture_file = NULL WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("error clearing avatar path in DB: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// No updated row
	}

	return nil
}
