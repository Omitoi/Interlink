package graph

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	"gitea.kood.tech/petrkubec/match-me/backend/graph/model"
	"github.com/golang-jwt/jwt/v5"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test database setup
func setupTestDB(t *testing.T) *sql.DB {
	// Use Docker database connection for testing
	db, err := sql.Open("postgres", "host=localhost port=5433 user=matchme_user password=matchme_password dbname=matchme_db sslmode=disable")
	require.NoError(t, err)

	err = db.Ping()
	require.NoError(t, err)

	return db
}

// Helper to create test context with user ID
func createTestContext(userID int) context.Context {
	return context.WithValue(context.Background(), userIDKey, userID)
}

// Helper to create test JWT token
func createTestToken(userID int) string {
	// Set a test JWT secret
	SetJWTSecret([]byte("test-secret-key-for-testing"))

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"expires": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, _ := token.SignedString([]byte("test-secret-key-for-testing"))
	return tokenString
}

func TestAuthenticationResolvers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	resolver := NewResolver(db)

	t.Run("Register", func(t *testing.T) {
		ctx := context.Background()
		email := "test@resolver.com"
		password := "testpassword123"

		result, err := resolver.Mutation().Register(ctx, email, password)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.NotEmpty(t, result.Token)
		assert.Equal(t, email, result.User.Email)
		assert.NotEmpty(t, result.User.ID)

		// Clean up - delete the test user
		defer func() {
			db.Exec("DELETE FROM users WHERE email = $1", email)
		}()
	})

	t.Run("Login", func(t *testing.T) {
		ctx := context.Background()
		email := "test-login@resolver.com"
		password := "testpassword123"

		// First register a user
		_, err := resolver.Mutation().Register(ctx, email, password)
		require.NoError(t, err)

		// Then login
		result, err := resolver.Mutation().Login(ctx, email, password)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.NotEmpty(t, result.Token)
		assert.Equal(t, email, result.User.Email)

		// Clean up
		defer func() {
			db.Exec("DELETE FROM users WHERE email = $1", email)
		}()
	})

	t.Run("Login Invalid Credentials", func(t *testing.T) {
		ctx := context.Background()

		result, err := resolver.Mutation().Login(ctx, "nonexistent@test.com", "wrongpassword")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "invalid credentials")
	})

	t.Run("Logout", func(t *testing.T) {
		ctx := context.Background()

		result, err := resolver.Mutation().Logout(ctx)
		require.NoError(t, err)
		assert.True(t, result)
	})
}

func TestUserResolvers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	resolver := NewResolver(db)

	// Create a test user first
	ctx := context.Background()
	email := "test-user@resolver.com"
	password := "testpassword123"

	registerResult, err := resolver.Mutation().Register(ctx, email, password)
	require.NoError(t, err)

	userID := registerResult.User.ID

	defer func() {
		// Clean up
		db.Exec("DELETE FROM profiles WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}()

	t.Run("User Query", func(t *testing.T) {
		result, err := resolver.Query().User(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, userID, result.ID)
		assert.Equal(t, email, result.Email)
	})

	t.Run("Me Query", func(t *testing.T) {
		// Create context with the actual user ID from the created user
		userIDInt, err := strconv.Atoi(userID)
		require.NoError(t, err)
		authCtx := createTestContext(userIDInt)

		result, err := resolver.Query().Me(authCtx)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, userID, result.ID)
	})

	t.Run("Me Query Unauthenticated", func(t *testing.T) {
		result, err := resolver.Query().Me(ctx)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "authentication required")
	})
}

func TestProfileResolvers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	resolver := NewResolver(db)

	// Create a test user first
	ctx := context.Background()
	email := "test-profile@resolver.com"
	password := "testpassword123"

	registerResult, err := resolver.Mutation().Register(ctx, email, password)
	require.NoError(t, err)

	userID := registerResult.User.ID
	authCtx := createTestContext(int(parseUserID(userID)))

	defer func() {
		// Clean up
		db.Exec("DELETE FROM profiles WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}()

	t.Run("UpdateProfile Create New", func(t *testing.T) {
		displayName := "Test User"
		aboutMe := "Test about me"
		locationCity := "Test City"

		input := model.ProfileInput{
			DisplayName:  &displayName,
			AboutMe:      &aboutMe,
			LocationCity: &locationCity,
		}

		result, err := resolver.Mutation().UpdateProfile(authCtx, input)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, displayName, result.DisplayName)
		assert.Equal(t, &aboutMe, result.AboutMe)
		assert.Equal(t, &locationCity, result.LocationCity)
		assert.True(t, result.IsComplete)
	})

	t.Run("UpdateProfile Update Existing", func(t *testing.T) {
		newDisplayName := "Updated Test User"
		newAboutMe := "Updated about me"

		input := model.ProfileInput{
			DisplayName: &newDisplayName,
			AboutMe:     &newAboutMe,
		}

		result, err := resolver.Mutation().UpdateProfile(authCtx, input)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, newDisplayName, result.DisplayName)
		assert.Equal(t, &newAboutMe, result.AboutMe)
		assert.True(t, result.IsComplete)
	})

	t.Run("MyProfile Query", func(t *testing.T) {
		result, err := resolver.Query().MyProfile(authCtx)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, userID, result.UserID)
		assert.NotEmpty(t, result.DisplayName)
	})

	t.Run("UserProfile Query", func(t *testing.T) {
		result, err := resolver.Query().UserProfile(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, userID, result.UserID)
	})
}

func TestNestedResolvers(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	resolver := NewResolver(db)

	// Create a test user with profile
	ctx := context.Background()
	email := "test-nested@resolver.com"
	password := "testpassword123"

	registerResult, err := resolver.Mutation().Register(ctx, email, password)
	require.NoError(t, err)

	userID := registerResult.User.ID
	authCtx := createTestContext(int(parseUserID(userID)))

	// Create a profile
	displayName := "Nested Test User"
	aboutMe := "Nested test about me"

	input := model.ProfileInput{
		DisplayName: &displayName,
		AboutMe:     &aboutMe,
	}

	_, err = resolver.Mutation().UpdateProfile(authCtx, input)
	require.NoError(t, err)

	defer func() {
		// Clean up
		db.Exec("DELETE FROM profiles WHERE user_id = $1", userID)
		db.Exec("DELETE FROM users WHERE id = $1", userID)
	}()

	t.Run("User Profile Relationship", func(t *testing.T) {
		user, err := resolver.Query().User(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, user)

		profile, err := resolver.User().Profile(ctx, user)
		require.NoError(t, err)
		require.NotNil(t, profile)

		assert.Equal(t, displayName, profile.DisplayName)
		assert.Equal(t, &aboutMe, profile.AboutMe)
	})

	t.Run("Profile User Relationship", func(t *testing.T) {
		profile, err := resolver.Query().UserProfile(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, profile)

		user, err := resolver.Profile().User(ctx, profile)
		require.NoError(t, err)
		require.NotNil(t, user)

		assert.Equal(t, userID, user.ID)
		assert.Equal(t, email, user.Email)
	})
}

// Helper function to parse user ID string to int
func parseUserID(userID string) int {
	id, err := strconv.Atoi(userID)
	if err != nil {
		return 1 // Default for testing
	}
	return id
}
