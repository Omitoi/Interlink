package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"gitea.kood.tech/petrkubec/match-me/backend/graph/model"
	"github.com/graph-gophers/dataloader/v7"
)

// DataLoaderContextKey is the key used to store dataloaders in context
type DataLoaderContextKey string

const dataLoaderKey DataLoaderContextKey = "dataloader"

// DataLoaders holds all the dataloaders for the application
type DataLoaders struct {
	UserLoader    *dataloader.Loader[int, *model.User]
	ProfileLoader *dataloader.Loader[int, *model.Profile]
	BioLoader     *dataloader.Loader[int, *model.Bio]
}

// NewDataLoaders creates new dataloaders with the database connection
func NewDataLoaders(db *sql.DB) *DataLoaders {
	return &DataLoaders{
		UserLoader:    dataloader.NewBatchedLoader(userBatchFn(db), dataloader.WithWait[int, *model.User](16*time.Millisecond)),
		ProfileLoader: dataloader.NewBatchedLoader(profileBatchFn(db), dataloader.WithWait[int, *model.Profile](16*time.Millisecond)),
		BioLoader:     dataloader.NewBatchedLoader(bioBatchFn(db), dataloader.WithWait[int, *model.Bio](16*time.Millisecond)),
	}
}

// GetDataLoadersFromContext retrieves dataloaders from context
func GetDataLoadersFromContext(ctx context.Context) *DataLoaders {
	if dl, ok := ctx.Value(dataLoaderKey).(*DataLoaders); ok {
		return dl
	}
	return nil
}

// WithDataLoaders adds dataloaders to context
func WithDataLoaders(ctx context.Context, dl *DataLoaders) context.Context {
	return context.WithValue(ctx, dataLoaderKey, dl)
}

// userBatchFn creates a batch function for loading users
func userBatchFn(db *sql.DB) dataloader.BatchFunc[int, *model.User] {
	return func(ctx context.Context, keys []int) []*dataloader.Result[*model.User] {
		results := make([]*dataloader.Result[*model.User], len(keys))

		// Create a map to track which keys we're looking for
		keyMap := make(map[int]int) // userID -> index in results
		for i, key := range keys {
			keyMap[key] = i
			results[i] = &dataloader.Result[*model.User]{} // Initialize all results
		}

		if len(keys) == 0 {
			return results
		}

		// Build placeholders for the IN clause
		placeholders := make([]string, len(keys))
		args := make([]interface{}, len(keys))
		for i, key := range keys {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = key
		}

		query := fmt.Sprintf(`
			SELECT id, email, created_at, updated_at, last_online 
			FROM users 
			WHERE id IN (%s)
		`, placeholders[0])

		for i := 1; i < len(placeholders); i++ {
			query = fmt.Sprintf("%s, %s", query[:len(query)-1], placeholders[i]) + ")"
		}

		// Fix the query format
		query = fmt.Sprintf(`
			SELECT id, email, created_at, updated_at, last_online 
			FROM users 
			WHERE id IN (%s)
		`, joinPlaceholders(placeholders))

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			// Set error for all results
			for i := range results {
				results[i].Error = err
			}
			return results
		}
		defer rows.Close()

		for rows.Next() {
			var user model.User
			var lastOnline sql.NullTime

			err := rows.Scan(
				&user.ID,
				&user.Email,
				&user.CreatedAt,
				&user.UpdatedAt,
				&lastOnline,
			)
			if err != nil {
				// Set error for all remaining results
				for i := range results {
					if results[i].Data == nil && results[i].Error == nil {
						results[i].Error = err
					}
				}
				return results
			}

			if lastOnline.Valid {
				formatted := lastOnline.Time.Format(time.RFC3339)
				user.LastOnline = &formatted
			}

			// Convert ID string to int for lookup
			userID, err := strconv.Atoi(user.ID)
			if err != nil {
				continue
			}

			// Find the correct position in results array
			if idx, ok := keyMap[userID]; ok {
				results[idx].Data = &user
			}
		}

		return results
	}
}

// profileBatchFn creates a batch function for loading profiles
func profileBatchFn(db *sql.DB) dataloader.BatchFunc[int, *model.Profile] {
	return func(ctx context.Context, keys []int) []*dataloader.Result[*model.Profile] {
		results := make([]*dataloader.Result[*model.Profile], len(keys))

		// Create a map to track which keys we're looking for
		keyMap := make(map[int]int) // userID -> index in results
		for i, key := range keys {
			keyMap[key] = i
			results[i] = &dataloader.Result[*model.Profile]{} // Initialize all results
		}

		if len(keys) == 0 {
			return results
		}

		// Build placeholders for the IN clause
		placeholders := make([]string, len(keys))
		args := make([]interface{}, len(keys))
		for i, key := range keys {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = key
		}

		query := fmt.Sprintf(`
			SELECT user_id, display_name, about_me, profile_picture_file, 
			       location_city, location_lat, location_lon, max_radius_km, is_complete
			FROM profiles 
			WHERE user_id IN (%s)
		`, joinPlaceholders(placeholders))

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			// Set error for all results
			for i := range results {
				results[i].Error = err
			}
			return results
		}
		defer rows.Close()

		for rows.Next() {
			var profile model.Profile
			var aboutMe, profilePictureFile, locationCity sql.NullString
			var locationLat, locationLon sql.NullFloat64
			var maxRadiusKm sql.NullInt32

			err := rows.Scan(
				&profile.UserID,
				&profile.DisplayName,
				&aboutMe,
				&profilePictureFile,
				&locationCity,
				&locationLat,
				&locationLon,
				&maxRadiusKm,
				&profile.IsComplete,
			)
			if err != nil {
				// Set error for all remaining results
				for i := range results {
					if results[i].Data == nil && results[i].Error == nil {
						results[i].Error = err
					}
				}
				return results
			}

			if aboutMe.Valid {
				profile.AboutMe = &aboutMe.String
			}
			if profilePictureFile.Valid {
				profile.ProfilePictureFile = &profilePictureFile.String
			}
			if locationCity.Valid {
				profile.LocationCity = &locationCity.String
			}
			if locationLat.Valid {
				profile.LocationLat = &locationLat.Float64
			}
			if locationLon.Valid {
				profile.LocationLon = &locationLon.Float64
			}
			if maxRadiusKm.Valid {
				intVal := int(maxRadiusKm.Int32)
				profile.MaxRadiusKm = &intVal
			}

			// Convert UserID string to int for lookup
			userID, err := strconv.Atoi(profile.UserID)
			if err != nil {
				continue
			}

			// Find the correct position in results array
			if idx, ok := keyMap[userID]; ok {
				results[idx].Data = &profile
			}
		}

		return results
	}
}

// bioBatchFn creates a batch function for loading bios
func bioBatchFn(db *sql.DB) dataloader.BatchFunc[int, *model.Bio] {
	return func(ctx context.Context, keys []int) []*dataloader.Result[*model.Bio] {
		results := make([]*dataloader.Result[*model.Bio], len(keys))

		// Create a map to track which keys we're looking for
		keyMap := make(map[int]int) // userID -> index in results
		for i, key := range keys {
			keyMap[key] = i
			results[i] = &dataloader.Result[*model.Bio]{} // Initialize all results
		}

		if len(keys) == 0 {
			return results
		}

		// Build placeholders for the IN clause
		placeholders := make([]string, len(keys))
		args := make([]interface{}, len(keys))
		for i, key := range keys {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			args[i] = key
		}

		query := fmt.Sprintf(`
			SELECT user_id, analog_passions, digital_delights, collaboration_interests, 
			       favorite_food, favorite_music
			FROM bios 
			WHERE user_id IN (%s)
		`, joinPlaceholders(placeholders))

		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			// Set error for all results
			for i := range results {
				results[i].Error = err
			}
			return results
		}
		defer rows.Close()

		for rows.Next() {
			var bio model.Bio
			var analogPassionsJSON, digitalDelightsJSON string
			var collaborationInterests, favoriteFood, favoriteMusic sql.NullString

			err := rows.Scan(
				&bio.UserID,
				&analogPassionsJSON,
				&digitalDelightsJSON,
				&collaborationInterests,
				&favoriteFood,
				&favoriteMusic,
			)
			if err != nil {
				// Set error for all remaining results
				for i := range results {
					if results[i].Data == nil && results[i].Error == nil {
						results[i].Error = err
					}
				}
				return results
			}

			// Parse JSON arrays for analog passions and digital delights
			bio.AnalogPassions = parseStringArray(analogPassionsJSON)
			bio.DigitalDelights = parseStringArray(digitalDelightsJSON)

			if collaborationInterests.Valid {
				bio.CollaborationInterests = &collaborationInterests.String
			}
			if favoriteFood.Valid {
				bio.FavoriteFood = &favoriteFood.String
			}
			if favoriteMusic.Valid {
				bio.FavoriteMusic = &favoriteMusic.String
			}

			// Convert UserID string to int for lookup
			userID, err := strconv.Atoi(bio.UserID)
			if err != nil {
				continue
			}

			// Find the correct position in results array
			if idx, ok := keyMap[userID]; ok {
				results[idx].Data = &bio
			}
		}

		return results
	}
}

// Helper function to join placeholders for IN clause
func joinPlaceholders(placeholders []string) string {
	if len(placeholders) == 0 {
		return ""
	}
	result := placeholders[0]
	for i := 1; i < len(placeholders); i++ {
		result += ", " + placeholders[i]
	}
	return result
}

// Helper function to parse JSON string array
func parseStringArray(jsonStr string) []string {
	var result []string
	if jsonStr == "" {
		return result
	}
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		// If JSON parsing fails, return empty array
		return []string{}
	}
	return result
}
