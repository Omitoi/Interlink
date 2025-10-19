package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitea.kood.tech/petrkubec/match-me/backend/graph/model"
	"github.com/99designs/gqlgen/graphql"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// External reference to the global jwtSecret from main package
var jwtSecret []byte

// SetJWTSecret sets the JWT secret for the GraphQL resolvers
func SetJWTSecret(secret []byte) {
	jwtSecret = secret
}

// GraphQL helper functions for authentication
func extractUserIDFromContext(ctx context.Context) (int, error) {
	// First try to get from context (set by middleware)
	if userID, ok := ctx.Value(userIDKey).(int); ok && userID > 0 {
		return userID, nil
	}

	return 0, fmt.Errorf("authentication required")
}

func createJWTToken(userID int) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"expires": time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString(jwtSecret)
}

func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

func verifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// Helper function to get user by ID
func (r *Resolver) getUserByID(userID int) (*model.User, error) {
	var user model.User
	err := r.DB.QueryRow(`
		SELECT id, email, created_at, updated_at, last_online
		FROM users WHERE id = $1
	`, userID).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

// Resolver holds the database connection and other dependencies
type Resolver struct {
	DB *sql.DB
}

// NewResolver creates a new resolver with the database connection
func NewResolver(db *sql.DB) *Resolver {
	return &Resolver{DB: db}
}

// Context keys for user authentication (must match main package)
type contextKey string

const userIDKey contextKey = "userID"

// getUserIDFromContext extracts the user ID from GraphQL context
func getUserIDFromContext(ctx context.Context) (int, error) {
	if userID, ok := ctx.Value(userIDKey).(int); ok && userID > 0 {
		return userID, nil
	}
	return 0, fmt.Errorf("unauthorized: user not authenticated")
}

// AuthMiddleware adds user authentication to GraphQL context
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				return jwtSecret, nil
			})
			if err == nil && token.Valid {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					if userID, ok := claims["user_id"].(float64); ok {
						ctx := context.WithValue(r.Context(), userIDKey, int(userID))
						r = r.WithContext(ctx)
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// User is the resolver for the user field.
func (r *bioResolver) User(ctx context.Context, obj *model.Bio) (*model.User, error) {
	// Use DataLoader if available
	if dataloaders := GetDataLoadersFromContext(ctx); dataloaders != nil {
		userID, err := strconv.Atoi(obj.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		thunk := dataloaders.UserLoader.Load(ctx, userID)
		return thunk()
	}

	// Fallback to direct database query
	var user model.User
	var lastOnline sql.NullTime
	err := r.DB.QueryRow(`
		SELECT id, email, created_at, updated_at, last_online
		FROM users WHERE id = $1
	`, obj.UserID).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &lastOnline,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if lastOnline.Valid {
		formatted := lastOnline.Time.Format(time.RFC3339)
		user.LastOnline = &formatted
	}

	return &user, nil
}

// Register is the resolver for the register field.
func (r *mutationResolver) Register(ctx context.Context, email string, password string) (*model.AuthResult, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	// Hash the password
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Insert user into database
	var newID int
	err = r.DB.QueryRow(
		"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
		email, hashedPassword,
	).Scan(&newID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			return nil, fmt.Errorf("email already exists")
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Update last_online for the new user
	_, err = r.DB.Exec("UPDATE users SET last_online = NOW() WHERE id = $1", newID)
	if err != nil {
		// Log but don't fail registration
		fmt.Printf("Failed to update last_online for new user: %v\n", err)
	}

	// Generate JWT token
	token, err := createJWTToken(newID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Fetch the created user
	user, err := r.getUserByID(newID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch created user: %w", err)
	}

	return &model.AuthResult{
		Token: token,
		User:  user,
	}, nil
}

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, email string, password string) (*model.AuthResult, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password are required")
	}

	var userID int
	var passwordHash string
	err := r.DB.QueryRow("SELECT id, password_hash FROM users WHERE email = $1", email).Scan(&userID, &passwordHash)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	} else if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Verify password
	if err := verifyPassword(passwordHash, password); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last_online
	_, err = r.DB.Exec("UPDATE users SET last_online = NOW() WHERE id = $1", userID)
	if err != nil {
		fmt.Printf("Failed to update last_online: %v\n", err)
		// Don't fail login, just log the error
	}

	// Generate JWT token
	token, err := createJWTToken(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Fetch the user
	user, err := r.getUserByID(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	return &model.AuthResult{
		Token: token,
		User:  user,
	}, nil
}

// Logout is the resolver for the logout field.
func (r *mutationResolver) Logout(ctx context.Context) (bool, error) {
	// For JWT tokens, logout is typically handled client-side by removing the token
	// We could implement a token blacklist here if needed
	// For now, just return true as the client should remove the token
	return true, nil
}

// UpdateProfile is the resolver for the updateProfile field.
func (r *mutationResolver) UpdateProfile(ctx context.Context, input model.ProfileInput) (*model.Profile, error) {
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Check if profile exists, if not create it
	var existingProfile model.Profile
	err = r.DB.QueryRow(`
		SELECT user_id, display_name, about_me, profile_picture_file,
		       location_city, location_lat, location_lon, max_radius_km, is_complete
		FROM profiles WHERE user_id = $1
	`, userID).Scan(
		&existingProfile.UserID, &existingProfile.DisplayName, &existingProfile.AboutMe,
		&existingProfile.ProfilePictureFile, &existingProfile.LocationCity,
		&existingProfile.LocationLat, &existingProfile.LocationLon,
		&existingProfile.MaxRadiusKm, &existingProfile.IsComplete,
	)

	if err == sql.ErrNoRows {
		// Create new profile
		profile := &model.Profile{
			UserID: fmt.Sprintf("%d", userID),
		}

		// Set values from input
		if input.DisplayName != nil {
			profile.DisplayName = *input.DisplayName
		}
		if input.AboutMe != nil {
			profile.AboutMe = input.AboutMe
		}
		if input.LocationCity != nil {
			profile.LocationCity = input.LocationCity
		}
		if input.LocationLat != nil {
			profile.LocationLat = input.LocationLat
		}
		if input.LocationLon != nil {
			profile.LocationLon = input.LocationLon
		}
		if input.MaxRadiusKm != nil {
			profile.MaxRadiusKm = input.MaxRadiusKm
		}

		// Check if profile is complete
		profile.IsComplete = profile.DisplayName != ""

		err = r.DB.QueryRow(`
			INSERT INTO profiles (user_id, display_name, about_me, location_city, location_lat, location_lon, max_radius_km, is_complete)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING user_id, display_name, about_me, profile_picture_file, location_city, location_lat, location_lon, max_radius_km, is_complete
		`, userID, profile.DisplayName, profile.AboutMe, profile.LocationCity,
			profile.LocationLat, profile.LocationLon, profile.MaxRadiusKm, profile.IsComplete).Scan(
			&profile.UserID, &profile.DisplayName, &profile.AboutMe, &profile.ProfilePictureFile,
			&profile.LocationCity, &profile.LocationLat, &profile.LocationLon,
			&profile.MaxRadiusKm, &profile.IsComplete,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to create profile: %w", err)
		}

		return profile, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch existing profile: %w", err)
	}

	// Update existing profile
	updates := make([]string, 0)
	args := make([]interface{}, 0)
	argCount := 0

	if input.DisplayName != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("display_name = $%d", argCount))
		args = append(args, *input.DisplayName)
	}
	if input.AboutMe != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("about_me = $%d", argCount))
		args = append(args, *input.AboutMe)
	}
	if input.LocationCity != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("location_city = $%d", argCount))
		args = append(args, *input.LocationCity)
	}
	if input.LocationLat != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("location_lat = $%d", argCount))
		args = append(args, *input.LocationLat)
	}
	if input.LocationLon != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("location_lon = $%d", argCount))
		args = append(args, *input.LocationLon)
	}
	if input.MaxRadiusKm != nil {
		argCount++
		updates = append(updates, fmt.Sprintf("max_radius_km = $%d", argCount))
		args = append(args, *input.MaxRadiusKm)
	}

	if len(updates) == 0 {
		return &existingProfile, nil // No updates requested
	}

	// Add is_complete check
	argCount++
	updates = append(updates, fmt.Sprintf("is_complete = $%d", argCount))

	// Determine if profile is complete based on current values and updates
	displayName := existingProfile.DisplayName
	if input.DisplayName != nil {
		displayName = *input.DisplayName
	}
	isComplete := displayName != ""
	args = append(args, isComplete)

	// Add user_id for WHERE clause
	argCount++
	args = append(args, userID)

	query := fmt.Sprintf(`
		UPDATE profiles SET %s 
		WHERE user_id = $%d
		RETURNING user_id, display_name, about_me, profile_picture_file, location_city, location_lat, location_lon, max_radius_km, is_complete
	`, strings.Join(updates, ", "), argCount)

	var profile model.Profile
	err = r.DB.QueryRow(query, args...).Scan(
		&profile.UserID, &profile.DisplayName, &profile.AboutMe, &profile.ProfilePictureFile,
		&profile.LocationCity, &profile.LocationLat, &profile.LocationLon,
		&profile.MaxRadiusKm, &profile.IsComplete,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return &profile, nil
}

// UploadAvatar is the resolver for the uploadAvatar field.
func (r *mutationResolver) UploadAvatar(ctx context.Context, file graphql.Upload) (*model.Profile, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Validate file type
	if file.ContentType != "image/jpeg" {
		return nil, fmt.Errorf("only JPEG images are allowed")
	}

	// Read first 512 bytes to validate it's actually a JPEG
	buffer := make([]byte, 512)
	bytesRead, err := file.File.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Validate MIME type from file content
	contentType := http.DetectContentType(buffer[:bytesRead])
	if contentType != "image/jpeg" {
		return nil, fmt.Errorf("file content is not JPEG format")
	}

	// Reset file position
	if seeker, ok := file.File.(interface {
		Seek(int64, int) (int64, error)
	}); ok {
		seeker.Seek(0, 0)
	} else {
		return nil, fmt.Errorf("cannot reset file position")
	}

	// Create uploads directory if it doesn't exist
	avatarRoot := "./uploads/avatars"
	if err := os.MkdirAll(avatarRoot, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate filename
	filename := fmt.Sprintf("%d.jpg", currentUserID)
	dst := filepath.Join(avatarRoot, filename)
	tmp := dst + ".tmp"

	// Save file
	out, err := os.Create(tmp)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, file.File); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}
	out.Close()

	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return nil, fmt.Errorf("failed to finalize file: %w", err)
	}

	// Update database
	result, err := r.DB.Exec(`
		UPDATE profiles 
		SET profile_picture_file = $1 
		WHERE user_id = $2
	`, filename, currentUserID)
	if err != nil {
		// Remove the uploaded file if database update fails
		os.Remove(dst)
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("failed to check update result: %w", err)
	}
	if rowsAffected == 0 {
		// Remove the uploaded file if no profile exists
		os.Remove(dst)
		return nil, fmt.Errorf("profile not found or not initialized")
	}

	// Return updated profile
	var profile model.Profile
	err = r.DB.QueryRow(`
		SELECT user_id, display_name, about_me, profile_picture_file,
		       location_city, location_lat, location_lon, max_radius_km, is_complete
		FROM profiles WHERE user_id = $1
	`, currentUserID).Scan(
		&profile.UserID, &profile.DisplayName, &profile.AboutMe, &profile.ProfilePictureFile,
		&profile.LocationCity, &profile.LocationLat, &profile.LocationLon,
		&profile.MaxRadiusKm, &profile.IsComplete,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch updated profile: %w", err)
	}

	return &profile, nil
}

// UpdateBio is the resolver for the updateBio field.
func (r *mutationResolver) UpdateBio(ctx context.Context, input model.BioInput) (*model.Bio, error) {
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Check if bio exists, if not create it
	var existingBio model.Bio
	var analogPassions, digitalDelights sql.NullString
	var collaborationInterests, favoriteFood, favoriteMusic sql.NullString

	err = r.DB.QueryRow(`
		SELECT user_id, analog_passions, digital_delights, collaboration_interests,
		       favorite_food, favorite_music
		FROM profiles WHERE user_id = $1
	`, userID).Scan(
		&existingBio.UserID, &analogPassions, &digitalDelights, &collaborationInterests,
		&favoriteFood, &favoriteMusic,
	)

	// Set string values from NullString
	if collaborationInterests.Valid {
		existingBio.CollaborationInterests = &collaborationInterests.String
	}
	if favoriteFood.Valid {
		existingBio.FavoriteFood = &favoriteFood.String
	}
	if favoriteMusic.Valid {
		existingBio.FavoriteMusic = &favoriteMusic.String
	}

	// Parse existing JSON arrays
	if analogPassions.Valid && analogPassions.String != "" {
		json.Unmarshal([]byte(analogPassions.String), &existingBio.AnalogPassions)
	}
	if digitalDelights.Valid && digitalDelights.String != "" {
		json.Unmarshal([]byte(digitalDelights.String), &existingBio.DigitalDelights)
	}

	if err == sql.ErrNoRows {
		// Create profile first if it doesn't exist
		_, err = r.DB.Exec(`
			INSERT INTO profiles (user_id, display_name, is_complete) 
			VALUES ($1, '', false)
			ON CONFLICT (user_id) DO NOTHING
		`, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to create profile: %w", err)
		}

		// Set default values for new bio
		existingBio.UserID = fmt.Sprintf("%d", userID)
		existingBio.AnalogPassions = []string{}
		existingBio.DigitalDelights = []string{}
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch existing bio: %w", err)
	}

	// Update bio fields with input values
	if input.AnalogPassions != nil {
		existingBio.AnalogPassions = input.AnalogPassions
	}
	if input.DigitalDelights != nil {
		existingBio.DigitalDelights = input.DigitalDelights
	}
	if input.CollaborationInterests != nil {
		existingBio.CollaborationInterests = input.CollaborationInterests
	}
	if input.FavoriteFood != nil {
		existingBio.FavoriteFood = input.FavoriteFood
	}
	if input.FavoriteMusic != nil {
		existingBio.FavoriteMusic = input.FavoriteMusic
	}

	// Convert arrays to JSON
	analogPassionsJSON, err := json.Marshal(existingBio.AnalogPassions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal analog passions: %w", err)
	}
	digitalDelightsJSON, err := json.Marshal(existingBio.DigitalDelights)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal digital delights: %w", err)
	}

	// Update the profile with bio data
	err = r.DB.QueryRow(`
		UPDATE profiles SET 
			analog_passions = $2,
			digital_delights = $3,
			collaboration_interests = $4,
			favorite_food = $5,
			favorite_music = $6
		WHERE user_id = $1
		RETURNING user_id, analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music
	`, userID, string(analogPassionsJSON), string(digitalDelightsJSON),
		existingBio.CollaborationInterests, existingBio.FavoriteFood, existingBio.FavoriteMusic).Scan(
		&existingBio.UserID, &analogPassions, &digitalDelights, &collaborationInterests,
		&favoriteFood, &favoriteMusic,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to update bio: %w", err)
	}

	// Set the returned string values
	if collaborationInterests.Valid {
		existingBio.CollaborationInterests = &collaborationInterests.String
	}
	if favoriteFood.Valid {
		existingBio.FavoriteFood = &favoriteFood.String
	}
	if favoriteMusic.Valid {
		existingBio.FavoriteMusic = &favoriteMusic.String
	}

	// Parse the returned JSON arrays
	if analogPassions.Valid && analogPassions.String != "" {
		json.Unmarshal([]byte(analogPassions.String), &existingBio.AnalogPassions)
	}
	if digitalDelights.Valid && digitalDelights.String != "" {
		json.Unmarshal([]byte(digitalDelights.String), &existingBio.DigitalDelights)
	}

	return &existingBio, nil
}

// RequestConnection is the resolver for the requestConnection field.
func (r *mutationResolver) RequestConnection(ctx context.Context, targetUserID string) (*model.Connection, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert target user ID to int
	targetID, err := strconv.Atoi(targetUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid target user ID: %w", err)
	}

	// Check if target user exists and has complete profile
	var exists bool
	err = r.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN profiles p ON p.user_id = u.id
			WHERE u.id = $1 AND COALESCE(p.is_complete, FALSE) = TRUE
		)
	`, targetID).Scan(&exists)

	if err != nil || !exists {
		return nil, fmt.Errorf("target user not found or profile not complete")
	}

	// Check if connection already exists
	var connectionID int
	var status string
	err = r.DB.QueryRow(`
		SELECT id, status FROM connections
		WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)
	`, currentUserID, targetID).Scan(&connectionID, &status)

	if err == nil {
		// Connection already exists
		if status == "accepted" {
			return nil, fmt.Errorf("connection already accepted")
		}
		if status == "pending" {
			return nil, fmt.Errorf("connection request already pending")
		}
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Create new connection request
	err = r.DB.QueryRow(`
		INSERT INTO connections (user_id, target_user_id, status, created_at)
		VALUES ($1, $2, 'pending', NOW())
		RETURNING id
	`, currentUserID, targetID).Scan(&connectionID)

	if err != nil {
		return nil, fmt.Errorf("failed to create connection request: %w", err)
	}

	// Return the created connection
	connection := &model.Connection{
		ID:           fmt.Sprintf("%d", connectionID),
		UserID:       fmt.Sprintf("%d", currentUserID),
		TargetUserID: targetUserID,
		Status:       model.ConnectionStatusPending,
		CreatedAt:    time.Now().Format(time.RFC3339),
		UpdatedAt:    time.Now().Format(time.RFC3339),
	}

	// Broadcast connection update to subscribers
	go func() {
		subscriptionManager := GetSubscriptionManager()
		subscriptionManager.BroadcastConnectionUpdate(connection)
	}()

	return connection, nil
}

// RespondToConnection is the resolver for the respondToConnection field.
func (r *mutationResolver) RespondToConnection(ctx context.Context, connectionID string, accept bool) (*model.Connection, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert connection ID to int
	connID, err := strconv.Atoi(connectionID)
	if err != nil {
		return nil, fmt.Errorf("invalid connection ID: %w", err)
	}

	// Get the connection details
	var userID, targetUserID int
	var status string
	var createdAt time.Time
	err = r.DB.QueryRow(`
		SELECT user_id, target_user_id, status, created_at
		FROM connections
		WHERE id = $1
	`, connID).Scan(&userID, &targetUserID, &status, &createdAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("connection not found")
	} else if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}

	// Check if current user is the target of this connection request
	if targetUserID != currentUserID {
		return nil, fmt.Errorf("unauthorized: you can only respond to requests sent to you")
	}

	// Check if connection is in pending state
	if status != "pending" {
		return nil, fmt.Errorf("connection is not in pending state")
	}

	if accept {
		// Accept the connection
		var acceptedAt time.Time
		err = r.DB.QueryRow(`
			UPDATE connections
			SET status = 'accepted', accepted_at = NOW()
			WHERE id = $1
			RETURNING accepted_at
		`, connID).Scan(&acceptedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to accept connection: %w", err)
		}

		connection := &model.Connection{
			ID:           fmt.Sprintf("%d", connID),
			UserID:       fmt.Sprintf("%d", userID),
			TargetUserID: fmt.Sprintf("%d", targetUserID),
			Status:       model.ConnectionStatusAccepted,
			CreatedAt:    createdAt.Format(time.RFC3339),
			UpdatedAt:    acceptedAt.Format(time.RFC3339),
		}

		// Broadcast connection update to subscribers
		go func() {
			subscriptionManager := GetSubscriptionManager()
			subscriptionManager.BroadcastConnectionUpdate(connection)
		}()

		return connection, nil
	}

	// Reject/delete the connection
	_, err = r.DB.Exec(`DELETE FROM connections WHERE id = $1`, connID)
	if err != nil {
		return nil, fmt.Errorf("failed to reject connection: %w", err)
	}

	connection := &model.Connection{
		ID:           fmt.Sprintf("%d", connID),
		UserID:       fmt.Sprintf("%d", userID),
		TargetUserID: fmt.Sprintf("%d", targetUserID),
		Status:       model.ConnectionStatusDismissed,
		CreatedAt:    createdAt.Format(time.RFC3339),
		UpdatedAt:    createdAt.Format(time.RFC3339),
	}

	// Broadcast connection update to subscribers
	go func() {
		subscriptionManager := GetSubscriptionManager()
		subscriptionManager.BroadcastConnectionUpdate(connection)
	}()

	return connection, nil
}

// Disconnect is the resolver for the disconnect field.
func (r *mutationResolver) Disconnect(ctx context.Context, targetUserID string) (bool, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return false, err
	}

	// Convert target user ID to int
	targetID, err := strconv.Atoi(targetUserID)
	if err != nil {
		return false, fmt.Errorf("invalid target user ID: %w", err)
	}

	// Delete the connection
	result, err := r.DB.Exec(`
		DELETE FROM connections
		WHERE (user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1)
	`, currentUserID, targetID)

	if err != nil {
		return false, fmt.Errorf("failed to disconnect: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("failed to check disconnect result: %w", err)
	}

	return rowsAffected > 0, nil
}

// SendMessage is the resolver for the sendMessage field.
func (r *mutationResolver) SendMessage(ctx context.Context, targetUserID string, content string) (*model.ChatMessage, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert target user ID to int
	targetID, err := strconv.Atoi(targetUserID)
	if err != nil {
		return nil, fmt.Errorf("invalid target user ID: %w", err)
	}

	// Validate that content is not empty
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("message content cannot be empty")
	}

	// Start transaction
	tx, err := r.DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Check that users are connected
	var connectionExists bool
	err = tx.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM connections
			WHERE status = 'accepted'
				AND ((user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1))
		)
	`, currentUserID, targetID).Scan(&connectionExists)
	if err != nil {
		return nil, fmt.Errorf("failed to check connection: %w", err)
	}
	if !connectionExists {
		return nil, fmt.Errorf("no accepted connection with target user")
	}

	// Get or create chat
	var chatID int
	err = tx.QueryRow(`
		SELECT id
		FROM chats
		WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
	`, currentUserID, targetID).Scan(&chatID)

	if err == sql.ErrNoRows {
		// Create new chat
		err = tx.QueryRow(`
			INSERT INTO chats (user1_id, user2_id)
			VALUES (LEAST($1::int, $2::int), GREATEST($1::int, $2::int))
			RETURNING id
		`, currentUserID, targetID).Scan(&chatID)
		if err != nil {
			return nil, fmt.Errorf("failed to create chat: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("failed to get chat: %w", err)
	}

	// Insert message
	var msgID int64
	var createdAt time.Time
	err = tx.QueryRow(`
		INSERT INTO messages (chat_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, chatID, currentUserID, content).Scan(&msgID, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// Update chat last_message_at and unread flags
	_, err = tx.Exec(`
		UPDATE chats c
		SET last_message_at = $3,
			unread_for_user1 = CASE WHEN $2 = c.user2_id THEN TRUE ELSE unread_for_user1 END,
			unread_for_user2 = CASE WHEN $2 = c.user1_id THEN TRUE ELSE unread_for_user2 END
		WHERE c.id = $1
	`, chatID, currentUserID, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to update chat: %w", err)
	}

	// Create ChatMessage response
	chatMessage := &model.ChatMessage{
		ID:        strconv.FormatInt(msgID, 10),
		ChatID:    strconv.Itoa(chatID),
		SenderID:  strconv.Itoa(currentUserID),
		Content:   content,
		CreatedAt: createdAt.Format(time.RFC3339),
		IsRead:    false, // New messages are unread by default
	}

	// Broadcast message to subscribers after successful transaction commit
	if err == nil { // Transaction will be committed
		go func() {
			// Broadcast to real-time subscribers
			subscriptionManager := GetSubscriptionManager()
			subscriptionManager.BroadcastMessage(chatMessage)
		}()
	}

	return chatMessage, nil
}

// MarkMessagesAsRead is the resolver for the markMessagesAsRead field.
func (r *mutationResolver) MarkMessagesAsRead(ctx context.Context, chatID string) (bool, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return false, err
	}

	// Convert chat ID to int
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		return false, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Verify that the current user is part of this chat
	var userIsInChat bool
	err = r.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM chats
			WHERE id = $1 AND (user1_id = $2 OR user2_id = $2)
		)
	`, chatIDInt, currentUserID).Scan(&userIsInChat)

	if err != nil {
		return false, fmt.Errorf("failed to verify chat access: %w", err)
	}
	if !userIsInChat {
		return false, fmt.Errorf("user not part of this chat")
	}

	// Start transaction
	tx, err := r.DB.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	// Mark messages as read (only messages not sent by current user)
	_, err = tx.Exec(`
		UPDATE messages
		SET is_read = TRUE
		WHERE chat_id = $1 AND sender_id <> $2 AND is_read = FALSE
	`, chatIDInt, currentUserID)
	if err != nil {
		return false, fmt.Errorf("failed to mark messages as read: %w", err)
	}

	// Clear unread flag for current user in this chat
	_, err = tx.Exec(`
		UPDATE chats c
		SET unread_for_user1 = CASE WHEN $2 = c.user1_id THEN FALSE ELSE unread_for_user1 END,
			unread_for_user2 = CASE WHEN $2 = c.user2_id THEN FALSE ELSE unread_for_user2 END
		WHERE c.id = $1
	`, chatIDInt, currentUserID)
	if err != nil {
		return false, fmt.Errorf("failed to clear unread flags: %w", err)
	}

	return true, nil
}

// DismissRecommendation is the resolver for the dismissRecommendation field.
func (r *mutationResolver) DismissRecommendation(ctx context.Context, userID string) (bool, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return false, err
	}

	// Insert into dismissed_recommendations table
	_, err = r.DB.Exec(`
		INSERT INTO dismissed_recommendations (user_id, dismissed_user_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id, dismissed_user_id) DO NOTHING
	`, currentUserID, userID)

	if err != nil {
		return false, fmt.Errorf("failed to dismiss recommendation: %w", err)
	}

	return true, nil
}

// User is the resolver for the user field.
func (r *profileResolver) User(ctx context.Context, obj *model.Profile) (*model.User, error) {
	// Use DataLoader if available
	if dataloaders := GetDataLoadersFromContext(ctx); dataloaders != nil {
		userID, err := strconv.Atoi(obj.UserID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		thunk := dataloaders.UserLoader.Load(ctx, userID)
		return thunk()
	}

	// Fallback to direct database query
	var user model.User
	var lastOnline sql.NullTime
	err := r.DB.QueryRow(`
		SELECT id, email, created_at, updated_at, last_online
		FROM users WHERE id = $1
	`, obj.UserID).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &lastOnline,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if lastOnline.Valid {
		formatted := lastOnline.Time.Format(time.RFC3339)
		user.LastOnline = &formatted
	}

	return &user, nil
}

// Me is the resolver for the me field.
func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return r.getUserByID(userID)
}

// User is the resolver for the user field.
func (r *queryResolver) User(ctx context.Context, id string) (*model.User, error) {
	var user model.User
	err := r.DB.QueryRow(`
		SELECT id, email, created_at, updated_at, last_online
		FROM users WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.LastOnline,
	)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return &user, nil
}

// MyProfile is the resolver for the myProfile field.
func (r *queryResolver) MyProfile(ctx context.Context) (*model.Profile, error) {
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return r.UserProfile(ctx, fmt.Sprintf("%d", userID))
}

// UserProfile is the resolver for the userProfile field.
func (r *queryResolver) UserProfile(ctx context.Context, id string) (*model.Profile, error) {
	var profile model.Profile
	err := r.DB.QueryRow(`
		SELECT user_id, display_name, about_me, profile_picture_file,
		       location_city, location_lat, location_lon, max_radius_km, is_complete
		FROM profiles WHERE user_id = $1
	`, id).Scan(
		&profile.UserID, &profile.DisplayName, &profile.AboutMe, &profile.ProfilePictureFile,
		&profile.LocationCity, &profile.LocationLat, &profile.LocationLon,
		&profile.MaxRadiusKm, &profile.IsComplete,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No profile
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}

	return &profile, nil
}

// MyBio is the resolver for the myBio field.
func (r *queryResolver) MyBio(ctx context.Context) (*model.Bio, error) {
	userID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	return r.UserBio(ctx, fmt.Sprintf("%d", userID))
} // UserBio is the resolver for the userBio field.
func (r *queryResolver) UserBio(ctx context.Context, id string) (*model.Bio, error) {
	var bio model.Bio
	var analogPassions, digitalDelights string

	err := r.DB.QueryRow(`
		SELECT user_id, analog_passions, digital_delights, collaboration_interests,
		       favorite_food, favorite_music
		FROM profiles WHERE user_id = $1
	`, id).Scan(
		&bio.UserID, &analogPassions, &digitalDelights, &bio.CollaborationInterests,
		&bio.FavoriteFood, &bio.FavoriteMusic,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No bio
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bio: %w", err)
	}

	// Parse JSON arrays
	if analogPassions != "" {
		json.Unmarshal([]byte(analogPassions), &bio.AnalogPassions)
	}
	if digitalDelights != "" {
		json.Unmarshal([]byte(digitalDelights), &bio.DigitalDelights)
	}

	return &bio, nil
}

// Recommendations is the resolver for the recommendations field.
func (r *queryResolver) Recommendations(ctx context.Context) ([]*model.User, error) {
	// Return some sample users for testing (no auth required for demo)
	rows, err := r.DB.Query(`
		SELECT DISTINCT u.id, u.email, u.created_at, u.updated_at, u.last_online
		FROM users u
		JOIN profiles p ON u.id = p.user_id
		WHERE p.is_complete = true
		ORDER BY u.last_online DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch recommendations: %w", err)
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var user model.User
		err := rows.Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &user.LastOnline)
		if err != nil {
			continue // Skip if scan fails
		}
		users = append(users, &user)
	}

	return users, nil
} // Connections is the resolver for the connections field.
func (r *queryResolver) Connections(ctx context.Context) ([]*model.Connection, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.Query(`
		SELECT c.id, c.user_id, c.target_user_id, c.status, c.created_at,
		       u1.id as user_id_full, u1.email as user_email,
		       u2.id as target_user_id_full, u2.email as target_user_email
		FROM connections c
		JOIN users u1 ON u1.id = c.user_id
		JOIN users u2 ON u2.id = c.target_user_id
		WHERE (c.user_id = $1 OR c.target_user_id = $1) AND c.status = 'accepted'
		ORDER BY c.created_at DESC
	`, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch connections: %w", err)
	}
	defer rows.Close()

	var connections []*model.Connection
	for rows.Next() {
		var conn model.Connection
		var userID, targetUserID int
		var userIDFull, targetUserIDFull int
		var userEmail, targetUserEmail string
		var createdAt time.Time

		err := rows.Scan(&conn.ID, &userID, &targetUserID, &conn.Status, &createdAt,
			&userIDFull, &userEmail, &targetUserIDFull, &targetUserEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connection: %w", err)
		}

		conn.UserID = strconv.Itoa(userID)
		conn.TargetUserID = strconv.Itoa(targetUserID)
		conn.CreatedAt = createdAt.Format(time.RFC3339)

		// Populate User and TargetUser fields
		conn.User = &model.User{
			ID:    strconv.Itoa(userIDFull),
			Email: userEmail,
		}
		conn.TargetUser = &model.User{
			ID:    strconv.Itoa(targetUserIDFull),
			Email: targetUserEmail,
		}

		connections = append(connections, &conn)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating connections: %w", err)
	}

	return connections, nil
}

// ConnectionRequests is the resolver for the connectionRequests field.
func (r *queryResolver) ConnectionRequests(ctx context.Context) ([]*model.Connection, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.Query(`
		SELECT c.id, c.user_id, c.target_user_id, c.status, c.created_at,
		       u1.id as user_id_full, u1.email as user_email,
		       u2.id as target_user_id_full, u2.email as target_user_email
		FROM connections c
		JOIN users u1 ON u1.id = c.user_id
		JOIN users u2 ON u2.id = c.target_user_id
		WHERE c.target_user_id = $1 AND c.status = 'pending'
		ORDER BY c.created_at DESC
	`, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch connection requests: %w", err)
	}
	defer rows.Close()

	var connections []*model.Connection
	for rows.Next() {
		var conn model.Connection
		var userID, targetUserID int
		var userIDFull, targetUserIDFull int
		var userEmail, targetUserEmail string
		var createdAt time.Time

		err := rows.Scan(&conn.ID, &userID, &targetUserID, &conn.Status, &createdAt,
			&userIDFull, &userEmail, &targetUserIDFull, &targetUserEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to scan connection: %w", err)
		}

		conn.UserID = strconv.Itoa(userID)
		conn.TargetUserID = strconv.Itoa(targetUserID)
		conn.CreatedAt = createdAt.Format(time.RFC3339)

		// Populate User and TargetUser fields
		conn.User = &model.User{
			ID:    strconv.Itoa(userIDFull),
			Email: userEmail,
		}
		conn.TargetUser = &model.User{
			ID:    strconv.Itoa(targetUserIDFull),
			Email: targetUserEmail,
		}

		connections = append(connections, &conn)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating connections: %w", err)
	}

	return connections, nil
}

// Chats is the resolver for the chats field.
func (r *queryResolver) Chats(ctx context.Context) ([]*model.Chat, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := r.DB.Query(`
		SELECT c.id, c.user1_id, c.user2_id, c.last_message_at,
		       CASE WHEN c.user1_id = $1 THEN c.unread_for_user1 ELSE c.unread_for_user2 END as unread_for_current,
		       c.unread_for_user1, c.unread_for_user2
		FROM chats c
		WHERE c.user1_id = $1 OR c.user2_id = $1
		ORDER BY c.last_message_at DESC
	`, currentUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chats: %w", err)
	}
	defer rows.Close()

	var chats []*model.Chat
	for rows.Next() {
		var chat model.Chat
		var user1ID, user2ID int
		var lastMessageAt *time.Time
		var unreadForCurrent, unreadForUser1, unreadForUser2 bool

		err := rows.Scan(&chat.ID, &user1ID, &user2ID, &lastMessageAt, &unreadForCurrent, &unreadForUser1, &unreadForUser2)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chat: %w", err)
		}

		chat.User1id = strconv.Itoa(user1ID)
		chat.User2id = strconv.Itoa(user2ID)
		chat.UnreadForUser1 = unreadForUser1
		chat.UnreadForUser2 = unreadForUser2

		if lastMessageAt != nil {
			lastMsgAt := lastMessageAt.Format(time.RFC3339)
			chat.LastMessageAt = &lastMsgAt
		}

		chats = append(chats, &chat)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chats: %w", err)
	}

	return chats, nil
}

// Chat is the resolver for the chat field.
func (r *queryResolver) Chat(ctx context.Context, id string) (*model.Chat, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert chat ID to int
	chatID, err := strconv.Atoi(id)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	var chat model.Chat
	var user1ID, user2ID int
	var lastMessageAt *time.Time

	err = r.DB.QueryRow(`
		SELECT id, user1_id, user2_id, last_message_at, unread_for_user1, unread_for_user2
		FROM chats
		WHERE id = $1 AND (user1_id = $2 OR user2_id = $2)
	`, chatID, currentUserID).Scan(
		&chat.ID, &user1ID, &user2ID, &lastMessageAt, &chat.UnreadForUser1, &chat.UnreadForUser2,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("chat not found or access denied")
	} else if err != nil {
		return nil, fmt.Errorf("failed to fetch chat: %w", err)
	}

	chat.User1id = strconv.Itoa(user1ID)
	chat.User2id = strconv.Itoa(user2ID)

	if lastMessageAt != nil {
		lastMsgAt := lastMessageAt.Format(time.RFC3339)
		chat.LastMessageAt = &lastMsgAt
	}

	return &chat, nil
}

// ChatMessages is the resolver for the chatMessages field.
func (r *queryResolver) ChatMessages(ctx context.Context, chatID string, limit *int, offset *int) ([]*model.ChatMessage, error) {
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert chat ID to int
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Verify user has access to this chat
	var hasAccess bool
	err = r.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM chats 
			WHERE id = $1 AND (user1_id = $2 OR user2_id = $2)
		)
	`, chatIDInt, currentUserID).Scan(&hasAccess)

	if err != nil {
		return nil, fmt.Errorf("failed to verify chat access: %w", err)
	}
	if !hasAccess {
		return nil, fmt.Errorf("chat not found or access denied")
	}

	// Set default values
	limitVal := 50
	if limit != nil && *limit > 0 && *limit <= 200 {
		limitVal = *limit
	}

	offsetVal := 0
	if offset != nil && *offset >= 0 {
		offsetVal = *offset
	}

	rows, err := r.DB.Query(`
		SELECT id, sender_id, content, created_at, is_read
		FROM messages
		WHERE chat_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, chatIDInt, limitVal, offsetVal)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch messages: %w", err)
	}
	defer rows.Close()

	var messages []*model.ChatMessage
	for rows.Next() {
		var msg model.ChatMessage
		var senderID int
		var createdAt time.Time

		err := rows.Scan(&msg.ID, &senderID, &msg.Content, &createdAt, &msg.IsRead)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}

		msg.ChatID = chatID
		msg.SenderID = strconv.Itoa(senderID)
		msg.CreatedAt = createdAt.Format(time.RFC3339)

		messages = append(messages, &msg)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

// MessageReceived is the resolver for the messageReceived field.
func (r *subscriptionResolver) MessageReceived(ctx context.Context, chatID string) (<-chan *model.ChatMessage, error) {
	// Verify user has access to this chat
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert chat ID to int for verification
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Verify user has access to this chat
	var hasAccess bool
	err = r.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM chats 
			WHERE id = $1 AND (user1_id = $2 OR user2_id = $2)
		)
	`, chatIDInt, currentUserID).Scan(&hasAccess)

	if err != nil {
		return nil, fmt.Errorf("failed to verify chat access: %w", err)
	}
	if !hasAccess {
		return nil, fmt.Errorf("chat not found or access denied")
	}

	// Subscribe to messages for this chat
	subscriptionManager := GetSubscriptionManager()
	ch, cleanup := subscriptionManager.SubscribeToMessages(chatID)

	// Handle context cancellation to cleanup subscription
	go func() {
		<-ctx.Done()
		cleanup()
	}()

	return ch, nil
}

// ConnectionUpdate is the resolver for the connectionUpdate field.
func (r *subscriptionResolver) ConnectionUpdate(ctx context.Context) (<-chan *model.Connection, error) {
	// Get current user ID
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Subscribe to connection updates for this user
	subscriptionManager := GetSubscriptionManager()
	ch, cleanup := subscriptionManager.SubscribeToConnections(fmt.Sprintf("%d", currentUserID))

	// Handle context cancellation to cleanup subscription
	go func() {
		<-ctx.Done()
		cleanup()
	}()

	return ch, nil
}

// UserPresence is the resolver for the userPresence field.
func (r *subscriptionResolver) UserPresence(ctx context.Context, userID string) (<-chan *model.PresenceUpdate, error) {
	// Verify that userID is valid
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	// Verify the target user exists
	var exists bool
	err := r.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil || !exists {
		return nil, fmt.Errorf("user not found")
	}

	// Subscribe to presence updates for this user
	subscriptionManager := GetSubscriptionManager()
	ch, cleanup := subscriptionManager.SubscribeToPresence(userID)

	// Handle context cancellation to cleanup subscription
	go func() {
		<-ctx.Done()
		cleanup()
	}()

	return ch, nil
}

// TypingStatus is the resolver for the typingStatus field.
func (r *subscriptionResolver) TypingStatus(ctx context.Context, chatID string) (<-chan *model.TypingStatus, error) {
	// Verify user has access to this chat
	currentUserID, err := extractUserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Convert chat ID to int for verification
	chatIDInt, err := strconv.Atoi(chatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Verify user has access to this chat
	var hasAccess bool
	err = r.DB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM chats 
			WHERE id = $1 AND (user1_id = $2 OR user2_id = $2)
		)
	`, chatIDInt, currentUserID).Scan(&hasAccess)

	if err != nil {
		return nil, fmt.Errorf("failed to verify chat access: %w", err)
	}
	if !hasAccess {
		return nil, fmt.Errorf("chat not found or access denied")
	}

	// Subscribe to typing status for this chat
	subscriptionManager := GetSubscriptionManager()
	ch, cleanup := subscriptionManager.SubscribeToTyping(chatID)

	// Handle context cancellation to cleanup subscription
	go func() {
		<-ctx.Done()
		cleanup()
	}()

	return ch, nil
}

// Profile is the resolver for the profile field.
func (r *userResolver) Profile(ctx context.Context, obj *model.User) (*model.Profile, error) {
	// Use DataLoader if available
	if dataloaders := GetDataLoadersFromContext(ctx); dataloaders != nil {
		userID, err := strconv.Atoi(obj.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		thunk := dataloaders.ProfileLoader.Load(ctx, userID)
		return thunk()
	}

	// Fallback to direct database query
	var profile model.Profile
	var aboutMe, profilePictureFile, locationCity sql.NullString
	var locationLat, locationLon sql.NullFloat64
	var maxRadiusKm sql.NullInt32

	err := r.DB.QueryRow(`
		SELECT user_id, display_name, about_me, profile_picture_file,
		       location_city, location_lat, location_lon, max_radius_km, is_complete
		FROM profiles WHERE user_id = $1
	`, obj.ID).Scan(
		&profile.UserID, &profile.DisplayName, &aboutMe, &profilePictureFile,
		&locationCity, &locationLat, &locationLon,
		&maxRadiusKm, &profile.IsComplete,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No profile
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch profile: %w", err)
	}

	// Handle nullable fields
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

	return &profile, nil
}

// Bio is the resolver for the bio field.
func (r *userResolver) Bio(ctx context.Context, obj *model.User) (*model.Bio, error) {
	// Use DataLoader if available
	if dataloaders := GetDataLoadersFromContext(ctx); dataloaders != nil {
		userID, err := strconv.Atoi(obj.ID)
		if err != nil {
			return nil, fmt.Errorf("invalid user ID: %w", err)
		}
		thunk := dataloaders.BioLoader.Load(ctx, userID)
		return thunk()
	}

	// Fallback to direct database query
	var bio model.Bio
	var analogPassions, digitalDelights string
	var collaborationInterests, favoriteFood, favoriteMusic sql.NullString

	err := r.DB.QueryRow(`
		SELECT user_id, analog_passions, digital_delights, collaboration_interests,
		       favorite_food, favorite_music
		FROM profiles WHERE user_id = $1
	`, obj.ID).Scan(
		&bio.UserID, &analogPassions, &digitalDelights, &collaborationInterests,
		&favoriteFood, &favoriteMusic,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No bio
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bio: %w", err)
	}

	// Handle nullable fields
	if collaborationInterests.Valid {
		bio.CollaborationInterests = &collaborationInterests.String
	}
	if favoriteFood.Valid {
		bio.FavoriteFood = &favoriteFood.String
	}
	if favoriteMusic.Valid {
		bio.FavoriteMusic = &favoriteMusic.String
	}

	// Parse JSON arrays
	if analogPassions != "" {
		json.Unmarshal([]byte(analogPassions), &bio.AnalogPassions)
	}
	if digitalDelights != "" {
		json.Unmarshal([]byte(digitalDelights), &bio.DigitalDelights)
	}

	return &bio, nil
}

// Bio returns BioResolver implementation.
func (r *Resolver) Bio() BioResolver { return &bioResolver{r} }

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Profile returns ProfileResolver implementation.
func (r *Resolver) Profile() ProfileResolver { return &profileResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

// Subscription returns SubscriptionResolver implementation.
func (r *Resolver) Subscription() SubscriptionResolver { return &subscriptionResolver{r} }

// User returns UserResolver implementation.
func (r *Resolver) User() UserResolver { return &userResolver{r} }

type bioResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type profileResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
