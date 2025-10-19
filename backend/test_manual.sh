#!/bin/bash

# GraphQL API Manual Testing Script
# This script demonstrates all the implemented GraphQL functionality

echo "=================================="
echo "   GraphQL API Testing Script    "
echo "=================================="
echo

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Base URL
GRAPHQL_URL="http://localhost:8080/graphql"

echo -e "${BLUE}Test 1: User Registration${NC}"
echo "Registering a new user..."
REGISTER_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"mutation { register(email: \"demo@graphql.com\", password: \"demopass123\") { token user { id email createdAt } } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$REGISTER_RESPONSE" | jq .
echo

# Extract token and user ID
TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r '.data.register.token')
USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.data.register.user.id')

echo -e "${BLUE}Test 2: User Login${NC}"
echo "Logging in with the registered user..."
LOGIN_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(email: \"demo@graphql.com\", password: \"demopass123\") { token user { id email lastOnline } } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$LOGIN_RESPONSE" | jq .
echo

# Use the login token for subsequent requests
TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.data.login.token')

echo -e "${BLUE}Test 3: Authenticated 'Me' Query${NC}"
echo "Querying current user info..."
ME_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"{ me { id email createdAt updatedAt } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$ME_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 4: Profile Creation/Update${NC}"
echo "Creating/updating user profile..."
PROFILE_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"mutation { updateProfile(input: { displayName: \"Demo GraphQL User\", aboutMe: \"This is a demo profile created via GraphQL API\", locationCity: \"GraphQL City\", maxRadiusKm: 50 }) { displayName aboutMe locationCity maxRadiusKm isComplete } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$PROFILE_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 5: My Profile Query${NC}"
echo "Querying my profile..."
MY_PROFILE_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"{ myProfile { displayName aboutMe locationCity maxRadiusKm isComplete } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$MY_PROFILE_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 6: Nested User Query${NC}"
echo "Querying user with nested profile data..."
NESTED_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d "{\"query\":\"{ user(id: \\\"$USER_ID\\\") { id email profile { displayName aboutMe isComplete } } }\"}" \
  $GRAPHQL_URL)

echo "Response:"
echo "$NESTED_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 7: Bio Query (should be null)${NC}"
echo "Querying user bio (should be null since we haven't set bio data)..."
BIO_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"{ myBio { analogPassions digitalDelights collaborationInterests } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$BIO_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 8: Public User Profile Query${NC}"
echo "Querying another user's profile (using known user ID 754)..."
PUBLIC_PROFILE_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"{ userProfile(id: \"754\") { displayName aboutMe isComplete } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$PUBLIC_PROFILE_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 9: Recommendations Query${NC}"
echo "Querying recommendations..."
RECOMMENDATIONS_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"{ recommendations { id email } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$RECOMMENDATIONS_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 10: Error Handling - Unauthenticated Request${NC}"
echo "Trying to access protected endpoint without token..."
UNAUTH_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"{ me { id email } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$UNAUTH_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 11: Error Handling - Invalid Login${NC}"
echo "Trying to login with invalid credentials..."
INVALID_LOGIN_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(email: \"nonexistent@test.com\", password: \"wrongpass\") { token user { id email } } }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$INVALID_LOGIN_RESPONSE" | jq .
echo

echo -e "${BLUE}Test 12: Logout${NC}"
echo "Logging out..."
LOGOUT_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"mutation { logout }"}' \
  $GRAPHQL_URL)

echo "Response:"
echo "$LOGOUT_RESPONSE" | jq .
echo

echo -e "${GREEN}=================================="
echo "   All Tests Completed!          "
echo "==================================${NC}"
echo
echo "Summary of Implemented Features:"
echo "✅ User Registration (register mutation)"
echo "✅ User Login (login mutation)"
echo "✅ User Logout (logout mutation)"
echo "✅ Protected Queries (me, myProfile, myBio)"
echo "✅ Public Queries (user, userProfile, userBio)"
echo "✅ Profile Management (updateProfile mutation)"
echo "✅ Nested Field Resolvers (User -> Profile -> User)"
echo "✅ Authentication Middleware"
echo "✅ Error Handling"
echo "✅ Recommendations Query"
echo "✅ JWT Token Management"
echo
echo -e "${BLUE}GraphQL Playground available at: http://localhost:8080/${NC}"
echo -e "${BLUE}API Documentation: Check schema.graphql${NC}"