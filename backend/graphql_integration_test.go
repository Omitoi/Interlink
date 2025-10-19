package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServerURL = "http://localhost:8080/graphql"

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors"`
}

type GraphQLError struct {
	Message string        `json:"message"`
	Path    []interface{} `json:"path"`
}

func sendGraphQLRequest(t *testing.T, query string, headers map[string]string) *GraphQLResponse {
	reqBody := GraphQLRequest{Query: query}
	jsonBody, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", testServerURL, bytes.NewBuffer(jsonBody))
	require.NoError(t, err)

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var graphqlResp GraphQLResponse
	err = json.Unmarshal(body, &graphqlResp)
	require.NoError(t, err)

	return &graphqlResp
}

func TestGraphQLIntegration(t *testing.T) {
	// Check if server is running before proceeding
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		t.Skip("Integration tests require the server to be running on localhost:8080. Run 'go run .' in a separate terminal first.")
		return
	}
	if resp != nil {
		resp.Body.Close()
	}

	// Wait for server to be ready
	time.Sleep(1 * time.Second)

	t.Run("Registration", func(t *testing.T) {
		query := `mutation { 
			register(email: "integration@test.com", password: "testpass123") { 
				token 
				user { id email } 
			} 
		}`

		resp := sendGraphQLRequest(t, query, nil)

		if len(resp.Errors) > 0 {
			t.Logf("GraphQL Errors: %+v", resp.Errors)
		}
		assert.Empty(t, resp.Errors, "Registration should not have errors")

		var data map[string]interface{}
		err := json.Unmarshal(resp.Data, &data)
		require.NoError(t, err)

		register := data["register"].(map[string]interface{})
		assert.NotEmpty(t, register["token"], "Token should be present")

		user := register["user"].(map[string]interface{})
		assert.Equal(t, "integration@test.com", user["email"])
		assert.NotEmpty(t, user["id"])
	})

	t.Run("Login", func(t *testing.T) {
		// First register a user
		registerQuery := `mutation { 
			register(email: "login@test.com", password: "testpass123") { 
				token 
				user { id email } 
			} 
		}`

		registerResp := sendGraphQLRequest(t, registerQuery, nil)
		require.Empty(t, registerResp.Errors)

		// Then login with the same credentials
		loginQuery := `mutation { 
			login(email: "login@test.com", password: "testpass123") { 
				token 
				user { id email } 
			} 
		}`

		loginResp := sendGraphQLRequest(t, loginQuery, nil)

		if len(loginResp.Errors) > 0 {
			t.Logf("Login GraphQL Errors: %+v", loginResp.Errors)
		}
		assert.Empty(t, loginResp.Errors, "Login should not have errors")

		var data map[string]interface{}
		err := json.Unmarshal(loginResp.Data, &data)
		require.NoError(t, err)

		login := data["login"].(map[string]interface{})
		assert.NotEmpty(t, login["token"], "Token should be present")

		user := login["user"].(map[string]interface{})
		assert.Equal(t, "login@test.com", user["email"])
	})

	t.Run("Invalid Login", func(t *testing.T) {
		query := `mutation { 
			login(email: "nonexistent@test.com", password: "wrongpass") { 
				token 
				user { id email } 
			} 
		}`

		resp := sendGraphQLRequest(t, query, nil)

		assert.NotEmpty(t, resp.Errors, "Invalid login should have errors")
		assert.Contains(t, resp.Errors[0].Message, "invalid credentials")
	})

	t.Run("Authenticated Me Query", func(t *testing.T) {
		// First register and get token
		registerQuery := `mutation { 
			register(email: "me@test.com", password: "testpass123") { 
				token 
				user { id email } 
			} 
		}`

		registerResp := sendGraphQLRequest(t, registerQuery, nil)
		require.Empty(t, registerResp.Errors)

		var registerData map[string]interface{}
		err := json.Unmarshal(registerResp.Data, &registerData)
		require.NoError(t, err)

		register := registerData["register"].(map[string]interface{})
		token := register["token"].(string)

		// Use token to query me
		meQuery := `{ me { id email } }`
		headers := map[string]string{
			"Authorization": "Bearer " + token,
		}

		meResp := sendGraphQLRequest(t, meQuery, headers)

		if len(meResp.Errors) > 0 {
			t.Logf("Me Query GraphQL Errors: %+v", meResp.Errors)
		}
		assert.Empty(t, meResp.Errors, "Me query should not have errors")

		var meData map[string]interface{}
		err = json.Unmarshal(meResp.Data, &meData)
		require.NoError(t, err)

		me := meData["me"].(map[string]interface{})
		assert.Equal(t, "me@test.com", me["email"])
	})

	t.Run("Profile Management", func(t *testing.T) {
		// Register user and get token
		registerQuery := `mutation { 
			register(email: "profile@test.com", password: "testpass123") { 
				token 
				user { id email } 
			} 
		}`

		registerResp := sendGraphQLRequest(t, registerQuery, nil)
		require.Empty(t, registerResp.Errors)

		var registerData map[string]interface{}
		err := json.Unmarshal(registerResp.Data, &registerData)
		require.NoError(t, err)

		register := registerData["register"].(map[string]interface{})
		token := register["token"].(string)
		headers := map[string]string{
			"Authorization": "Bearer " + token,
		}

		// Update profile
		updateQuery := `mutation { 
			updateProfile(input: { 
				displayName: "Test Profile User", 
				aboutMe: "Integration test profile",
				locationCity: "Test City"
			}) { 
				displayName 
				aboutMe 
				locationCity 
				isComplete 
			} 
		}`

		updateResp := sendGraphQLRequest(t, updateQuery, headers)

		if len(updateResp.Errors) > 0 {
			t.Logf("Update Profile GraphQL Errors: %+v", updateResp.Errors)
		}
		assert.Empty(t, updateResp.Errors, "Profile update should not have errors")

		var updateData map[string]interface{}
		err = json.Unmarshal(updateResp.Data, &updateData)
		require.NoError(t, err)

		profile := updateData["updateProfile"].(map[string]interface{})
		assert.Equal(t, "Test Profile User", profile["displayName"])
		assert.Equal(t, "Integration test profile", profile["aboutMe"])
		assert.Equal(t, "Test City", profile["locationCity"])
		assert.True(t, profile["isComplete"].(bool))

		// Query myProfile
		profileQuery := `{ myProfile { displayName aboutMe locationCity isComplete } }`

		profileResp := sendGraphQLRequest(t, profileQuery, headers)

		if len(profileResp.Errors) > 0 {
			t.Logf("My Profile GraphQL Errors: %+v", profileResp.Errors)
		}
		assert.Empty(t, profileResp.Errors, "My profile query should not have errors")

		var profileData map[string]interface{}
		err = json.Unmarshal(profileResp.Data, &profileData)
		require.NoError(t, err)

		myProfile := profileData["myProfile"].(map[string]interface{})
		assert.Equal(t, "Test Profile User", myProfile["displayName"])
		assert.Equal(t, "Integration test profile", myProfile["aboutMe"])
	})

	t.Run("Nested Queries", func(t *testing.T) {
		// Register user and create profile
		registerQuery := `mutation { 
			register(email: "nested@test.com", password: "testpass123") { 
				token 
				user { id email } 
			} 
		}`

		registerResp := sendGraphQLRequest(t, registerQuery, nil)
		require.Empty(t, registerResp.Errors)

		var registerData map[string]interface{}
		err := json.Unmarshal(registerResp.Data, &registerData)
		require.NoError(t, err)

		register := registerData["register"].(map[string]interface{})
		token := register["token"].(string)
		userID := register["user"].(map[string]interface{})["id"].(string)

		headers := map[string]string{
			"Authorization": "Bearer " + token,
		}

		// Create profile
		updateQuery := `mutation { 
			updateProfile(input: { 
				displayName: "Nested Test User", 
				aboutMe: "Nested test profile"
			}) { 
				displayName 
				aboutMe 
			} 
		}`

		updateResp := sendGraphQLRequest(t, updateQuery, headers)
		require.Empty(t, updateResp.Errors)

		// Query user with nested profile
		nestedQuery := fmt.Sprintf(`{ 
			user(id: "%s") { 
				id 
				email 
				profile { 
					displayName 
					aboutMe 
					isComplete 
				} 
			} 
		}`, userID)

		nestedResp := sendGraphQLRequest(t, nestedQuery, nil)

		if len(nestedResp.Errors) > 0 {
			t.Logf("Nested Query GraphQL Errors: %+v", nestedResp.Errors)
		}
		assert.Empty(t, nestedResp.Errors, "Nested query should not have errors")

		var nestedData map[string]interface{}
		err = json.Unmarshal(nestedResp.Data, &nestedData)
		require.NoError(t, err)

		user := nestedData["user"].(map[string]interface{})
		assert.Equal(t, "nested@test.com", user["email"])

		profile := user["profile"].(map[string]interface{})
		assert.Equal(t, "Nested Test User", profile["displayName"])
		assert.Equal(t, "Nested test profile", profile["aboutMe"])
		assert.True(t, profile["isComplete"].(bool))
	})

	t.Run("Unauthenticated Query", func(t *testing.T) {
		query := `{ me { id email } }`

		resp := sendGraphQLRequest(t, query, nil)

		assert.NotEmpty(t, resp.Errors, "Unauthenticated query should have errors")
		assert.Contains(t, resp.Errors[0].Message, "authentication required")
	})

	t.Run("Logout", func(t *testing.T) {
		// Register user and get token
		registerQuery := `mutation { 
			register(email: "logout@test.com", password: "testpass123") { 
				token 
			} 
		}`

		registerResp := sendGraphQLRequest(t, registerQuery, nil)
		require.Empty(t, registerResp.Errors)

		var registerData map[string]interface{}
		err := json.Unmarshal(registerResp.Data, &registerData)
		require.NoError(t, err)

		register := registerData["register"].(map[string]interface{})
		token := register["token"].(string)

		headers := map[string]string{
			"Authorization": "Bearer " + token,
		}

		// Logout
		logoutQuery := `mutation { logout }`

		logoutResp := sendGraphQLRequest(t, logoutQuery, headers)

		if len(logoutResp.Errors) > 0 {
			t.Logf("Logout GraphQL Errors: %+v", logoutResp.Errors)
		}
		assert.Empty(t, logoutResp.Errors, "Logout should not have errors")

		var logoutData map[string]interface{}
		err = json.Unmarshal(logoutResp.Data, &logoutData)
		require.NoError(t, err)

		logout := logoutData["logout"].(bool)
		assert.True(t, logout)
	})
}
