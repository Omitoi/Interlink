package main

import (
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

		displayName, profilePicture, err := fetchBasicUserInfo(db, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		// Check peer's online status
		onlineDB, err := isOnlineNow(db, userID) // TTL 90s in SQL
		if err != nil {
			// Not critical. If fails, assume that the user is offline
			onlineDB = false
		}

		resp := map[string]interface{}{
			"id":              userID,
			"display_name":    displayName,
			"profile_picture": profilePicture,
			"is_online":       onlineDB,
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// GET /users/{id}/profile
func userProfileHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		// Parse user ID from URL: /users/{id}/profile
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

		// Permission check: Only allow if recommended, pending/accepted connection, or connected
		allowed := false
		var count int
		err = db.QueryRow(`
            SELECT COUNT(*) FROM connections
            WHERE ((user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1))
            AND status IN ('accepted', 'pending')
        `, requesterID, targetID).Scan(&count)
		if err == nil && count > 0 {
			allowed = true
		}
		if !allowed {
			recs, err := getRecommendedUserIDs(db, requesterID)
			if err == nil {
				for _, id := range recs {
					if id == targetID {
						allowed = true
						break
					}
				}
			}
		}
		if !allowed {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		aboutMe, displayName, profilePicture, err := fetchProfileInfo(db, targetID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}

		// Fetch location data for distance calculation
		var locationLat, locationLon sql.NullFloat64
		err = db.QueryRow(`
			SELECT location_lat, location_lon 
			FROM profiles 
			WHERE user_id = $1
		`, targetID).Scan(&locationLat, &locationLon)
		if err != nil {
			// Location data is optional, continue without it
			locationLat.Valid = false
			locationLon.Valid = false
		}

		// Check peer's online status
		onlineDB, err := isOnlineNow(db, targetID) // TTL 90s in SQL
		if err != nil {
			// Not critical. If fails, assume that the user is offline
			onlineDB = false
		}

		resp := map[string]interface{}{
			"id":              targetID,
			"display_name":    displayName,
			"profile_picture": profilePicture,
			"about_me":        aboutMe,
			"is_online":       onlineDB,
		}

		// Add location data if available
		if locationLat.Valid {
			resp["location_lat"] = locationLat.Float64
		}
		if locationLon.Valid {
			resp["location_lon"] = locationLon.Float64
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

func userBioHandler(db *sql.DB) http.HandlerFunc {
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
		// Reuse permission strategy: connections or recommendations
		allowed := false
		var count int
		_ = db.QueryRow(`SELECT COUNT(*) FROM connections WHERE ((user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)) AND status IN ('accepted','pending')`, requesterID, targetID).Scan(&count)
		if count > 0 {
			allowed = true
		}
		if !allowed {
			if recs, err := getRecommendedUserIDs(db, requesterID); err == nil {
				for _, id := range recs {
					if id == targetID {
						allowed = true
						break
					}
				}
			}
		}
		if !allowed {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		var analog, digital, collaborationInterests, favoriteFood, favoriteMusic json.RawMessage
		err = db.QueryRow(`SELECT analog_passions, digital_delights, to_jsonb(collaboration_interests), to_jsonb(favorite_food), to_jsonb(favorite_music) FROM profiles WHERE user_id = $1`, targetID).Scan(&analog, &digital, &collaborationInterests, &favoriteFood, &favoriteMusic)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":                      targetID,
			"analog_passions":         jsonRawOrArray(analog),
			"digital_delights":        jsonRawOrArray(digital),
			"collaboration_interests": jsonRawOrArray(collaborationInterests),
			"favorite_food":           jsonRawOrArray(favoriteFood),
			"favorite_music":          jsonRawOrArray(favoriteMusic),
		})
	})
}

func completeProfileHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPatch {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		type ProfileRequest struct {
			DisplayName      string          `json:"display_name"`
			AboutMe          string          `json:"about_me"`
			ProfilePicture   string          `json:"profile_picture_file"`
			LocationCity     string          `json:"location_city"`
			LocationLat      float64         `json:"location_lat"`
			LocationLon      float64         `json:"location_lon"`
			MaxRadiusKm      int             `json:"max_radius_km"`
			AnalogPassions   json.RawMessage `json:"analog_passions"`
			DigitalDelights  json.RawMessage `json:"digital_delights"`
			CrossPollination string          `json:"collaboration_interests"`
			FavoriteFood     string          `json:"favorite_food"`
			FavoriteMusic    string          `json:"favorite_music"`
			OtherBio         json.RawMessage `json:"other_bio"`
			MatchPreferences json.RawMessage `json:"match_preferences"`
		}
		var req ProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json")
			return
		}
		userID := r.Context().Value(userIDKey).(int)

		_, err := db.Exec(`
            INSERT INTO profiles (
                user_id, display_name, about_me, location_city, location_lat, location_lon, max_radius_km,
                analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music, other_bio, match_preferences, is_complete
            ) VALUES (
                $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE
            )
            ON CONFLICT (user_id) DO UPDATE SET
                display_name = EXCLUDED.display_name,
                about_me = EXCLUDED.about_me,
                location_city = EXCLUDED.location_city,
                location_lat = EXCLUDED.location_lat,
                location_lon = EXCLUDED.location_lon,
                max_radius_km = EXCLUDED.max_radius_km,
                analog_passions = EXCLUDED.analog_passions,
                digital_delights = EXCLUDED.digital_delights,
                collaboration_interests = EXCLUDED.collaboration_interests,
                favorite_food = EXCLUDED.favorite_food,
                favorite_music = EXCLUDED.favorite_music,
                other_bio = EXCLUDED.other_bio,
                match_preferences = EXCLUDED.match_preferences,
                is_complete = TRUE
        `,
			userID, req.DisplayName, req.AboutMe, req.LocationCity, req.LocationLat, req.LocationLon, req.MaxRadiusKm,
			req.AnalogPassions, req.DigitalDelights, req.CrossPollination, req.FavoriteFood, req.FavoriteMusic, req.OtherBio, req.MatchPreferences,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "profile_save_error")
			log.Println("Error saving profile:", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
}

func meHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		displayName, profilePicture, err := fetchBasicUserInfo(db, userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		resp := map[string]interface{}{
			"id":              userID,
			"display_name":    displayName,
			"profile_picture": profilePicture,
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

// --- Self profile view handler (GET /me/profile) returning profile details ---
func meProfileHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "invalid_method")
			return
		}
		userID := r.Context().Value(userIDKey).(int)

		// Fetch all profile data
		var displayName, aboutMe, locationCity, crossPollination, favoriteFood, favoriteMusic string
		var profilePicture sql.NullString
		var locationLat, locationLon sql.NullFloat64
		var maxRadiusKm sql.NullInt64
		var analogPassions, digitalDelights, otherBio, matchPreferences json.RawMessage
		var isComplete sql.NullBool

		err := db.QueryRow(`
			SELECT display_name, about_me, profile_picture_file, location_city, location_lat, location_lon, 
			       max_radius_km, analog_passions, digital_delights, collaboration_interests, favorite_food, 
			       favorite_music, other_bio, match_preferences, is_complete
			FROM profiles WHERE user_id = $1
		`, userID).Scan(
			&displayName, &aboutMe, &profilePicture, &locationCity, &locationLat, &locationLon,
			&maxRadiusKm, &analogPassions, &digitalDelights, &crossPollination, &favoriteFood,
			&favoriteMusic, &otherBio, &matchPreferences, &isComplete,
		)

		if err != nil {
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound, "profile_not_found")
			} else {
				writeError(w, http.StatusInternalServerError, "database_error")
			}
			return
		}

		// Prepare response with all profile data
		profilePictureValue := ""
		if profilePicture.Valid {
			profilePictureValue = profilePicture.String
		}

		response := map[string]interface{}{
			"id":                      userID,
			"display_name":            displayName,
			"about_me":                aboutMe,
			"profile_picture":         profilePictureValue,
			"location_city":           locationCity,
			"collaboration_interests": crossPollination,
			"favorite_food":           favoriteFood,
			"favorite_music":          favoriteMusic,
			"is_complete":             isComplete.Bool,
		}

		// Handle nullable location fields
		if locationLat.Valid {
			response["location_lat"] = locationLat.Float64
		}
		if locationLon.Valid {
			response["location_lon"] = locationLon.Float64
		}
		if maxRadiusKm.Valid {
			response["max_radius_km"] = maxRadiusKm.Int64
		}

		// Handle JSON fields
		if analogPassions != nil {
			response["analog_passions"] = jsonRawOrArray(analogPassions)
		}
		if digitalDelights != nil {
			response["digital_delights"] = jsonRawOrArray(digitalDelights)
		}
		if otherBio != nil {
			var parsed interface{}
			if json.Unmarshal(otherBio, &parsed) == nil {
				response["other_bio"] = parsed
			}
		}
		if matchPreferences != nil {
			var parsed interface{}
			if json.Unmarshal(matchPreferences, &parsed) == nil {
				response["match_preferences"] = parsed
			}
		}

		writeJSON(w, http.StatusOK, response)
	})
}

// Bio handlers (GET /me/bio and GET /users/{id}/bio) - simplified placeholder extraction
func meBioHandler(db *sql.DB) http.HandlerFunc {
	return authenticate(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Context().Value(userIDKey).(int)
		var analog, digital, seeking, interests json.RawMessage
		err := db.QueryRow(`SELECT analog_passions, digital_delights, to_jsonb(collaboration_interests), to_jsonb(favorite_music) FROM profiles WHERE user_id = $1`, userID).Scan(&analog, &digital, &seeking, &interests)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":               userID,
			"analog_passions":  jsonRawOrArray(analog),
			"digital_delights": jsonRawOrArray(digital),
			"seeking":          jsonRawOrArray(seeking),
			"interests":        jsonRawOrArray(interests),
		})
	})
}
