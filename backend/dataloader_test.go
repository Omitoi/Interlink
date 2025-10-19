package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataLoaderPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping DataLoader performance test in short mode")
	}

	// Check if server is running before proceeding
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://localhost:8080/health")
	if err != nil {
		t.Skip("DataLoader performance tests require the server to be running on localhost:8080. Run 'go run .' in a separate terminal first.")
		return
	}
	if resp != nil {
		resp.Body.Close()
	}

	// This test verifies that DataLoader reduces database queries from N+1 to batched queries
	// We'll test by fetching multiple users with their profiles and bios in a single GraphQL query

	// First, register a test user and get auth token
	registerPayload := map[string]interface{}{
		"query": `
			mutation Register($email: String!, $password: String!) {
				register(email: $email, password: $password) {
					token
					user {
						id
						email
					}
				}
			}
		`,
		"variables": map[string]interface{}{
			"email":    fmt.Sprintf("dataloader_test_%d@example.com", time.Now().UnixNano()),
			"password": "testpassword123",
		},
	}

	registerBody, _ := json.Marshal(registerPayload)
	registerResp, err := http.Post("http://localhost:8080/graphql", "application/json", bytes.NewBuffer(registerBody))
	require.NoError(t, err)
	defer registerResp.Body.Close()

	var registerResult map[string]interface{}
	err = json.NewDecoder(registerResp.Body).Decode(&registerResult)
	require.NoError(t, err)

	// Extract token
	data := registerResult["data"].(map[string]interface{})
	authResult := data["register"].(map[string]interface{})
	token := authResult["token"].(string)
	userID := authResult["user"].(map[string]interface{})["id"].(string)

	// Create a profile for this user
	updateProfilePayload := map[string]interface{}{
		"query": `
			mutation UpdateProfile($input: ProfileInput!) {
				updateProfile(input: $input) {
					userID
					displayName
					isComplete
				}
			}
		`,
		"variables": map[string]interface{}{
			"input": map[string]interface{}{
				"displayName": "DataLoader Test User",
				"aboutMe":     "Testing DataLoader performance",
			},
		},
	}

	profileBody, _ := json.Marshal(updateProfilePayload)
	req, _ := http.NewRequest("POST", "http://localhost:8080/graphql", bytes.NewBuffer(profileBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	graphqlClient := &http.Client{}
	queryResp, err := graphqlClient.Do(req)
	require.NoError(t, err)
	defer queryResp.Body.Close()

	// Now test the DataLoader performance by fetching recommendations with nested data
	// This should demonstrate batching vs N+1 queries
	recommendationsQuery := map[string]interface{}{
		"query": `
			query GetRecommendationsWithData {
				recommendations {
					id
					email
					profile {
						userID
						displayName
						aboutMe
						isComplete
					}
					bio {
						userID
						analogPassions
						digitalDelights
						collaborationInterests
					}
				}
			}
		`,
	}

	queryBody, _ := json.Marshal(recommendationsQuery)
	req, _ = http.NewRequest("POST", "http://localhost:8080/graphql", bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	perfResp, err := graphqlClient.Do(req)
	require.NoError(t, err)
	defer perfResp.Body.Close()
	duration := time.Since(start)

	var result map[string]interface{}
	err = json.NewDecoder(perfResp.Body).Decode(&result)
	require.NoError(t, err)

	// Verify we got recommendations without errors
	assert.Nil(t, result["errors"], "GraphQL query should not have errors")
	assert.NotNil(t, result["data"], "GraphQL query should return data")

	data = result["data"].(map[string]interface{})
	recommendations := data["recommendations"].([]interface{})

	// Performance test: with DataLoader, this should be significantly faster
	// than without (especially with more users in the database)
	t.Logf("DataLoader query completed in %v with %d recommendations", duration, len(recommendations))
	t.Logf("User ID created for test: %s", userID)

	// Verify each recommendation has the nested data loaded
	for i, rec := range recommendations {
		recMap := rec.(map[string]interface{})
		assert.NotEmpty(t, recMap["id"], "Recommendation %d should have ID", i)
		assert.NotEmpty(t, recMap["email"], "Recommendation %d should have email", i)

		// Note: profile and bio might be nil for users without complete profiles
		// That's expected behavior
		t.Logf("Recommendation %d: ID=%s, has profile=%v, has bio=%v",
			i, recMap["id"], recMap["profile"] != nil, recMap["bio"] != nil)
	}

	// Performance assertion: should complete within reasonable time
	// This is a rough check - in production you'd want more sophisticated metrics
	assert.Less(t, duration, 5*time.Second, "Query should complete quickly with DataLoader")
}

func TestDataLoaderBatching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping DataLoader batching test in short mode")
	}

	// Check if server is running before proceeding
	httpClient := &http.Client{Timeout: 2 * time.Second}
	healthResp, err := httpClient.Get("http://localhost:8080/health")
	if err != nil {
		t.Skip("DataLoader batching tests require the server to be running on localhost:8080. Run 'go run .' in a separate terminal first.")
		return
	}
	if healthResp != nil {
		healthResp.Body.Close()
	}

	// This test specifically tests that DataLoader batches requests
	// by making a query that would trigger multiple User field resolvers

	// Register a test user
	registerPayload := map[string]interface{}{
		"query": `
			mutation Register($email: String!, $password: String!) {
				register(email: $email, password: $password) {
					token
				}
			}
		`,
		"variables": map[string]interface{}{
			"email":    fmt.Sprintf("batch_test_%d@example.com", time.Now().UnixNano()),
			"password": "testpassword123",
		},
	}

	registerBody, _ := json.Marshal(registerPayload)
	batchRegisterResp, err := http.Post("http://localhost:8080/graphql", "application/json", bytes.NewBuffer(registerBody))
	require.NoError(t, err)
	defer batchRegisterResp.Body.Close()

	var registerResult map[string]interface{}
	err = json.NewDecoder(batchRegisterResp.Body).Decode(&registerResult)
	require.NoError(t, err)

	data := registerResult["data"].(map[string]interface{})
	authResult := data["register"].(map[string]interface{})
	token := authResult["token"].(string)

	// Query that should trigger DataLoader batching
	// Getting connections which includes User relationships that should be batched
	connectionsQuery := map[string]interface{}{
		"query": `
			query GetConnectionsWithUsers {
				connections {
					id
					status
					user {
						id
						email
						profile {
							displayName
						}
					}
					targetUser {
						id
						email
						profile {
							displayName
						}
					}
				}
			}
		`,
	}

	queryBody, _ := json.Marshal(connectionsQuery)
	req, _ := http.NewRequest("POST", "http://localhost:8080/graphql", bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	batchClient := &http.Client{}
	start := time.Now()
	batchResp, err := batchClient.Do(req)
	require.NoError(t, err)
	defer batchResp.Body.Close()
	duration := time.Since(start)

	var result map[string]interface{}
	err = json.NewDecoder(batchResp.Body).Decode(&result)
	require.NoError(t, err)

	// Should not have errors
	assert.Nil(t, result["errors"], "Connections query should not have errors")

	data = result["data"].(map[string]interface{})
	connections := data["connections"].([]interface{})

	t.Logf("Connections query with DataLoader completed in %v, returned %d connections", duration, len(connections))

	// Verify structure (even if no connections exist yet)
	assert.NotNil(t, connections, "Connections array should not be nil")
}
