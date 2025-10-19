
# Test 1: Registration
REGISTER_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"mutation { register(email: \"test1@graphql.com\", password: \"password123\") { token user { id email } } }"}' \
  http://localhost:8080/graphql)

echo "=== Registration Test ==="
echo "$REGISTER_RESPONSE" | jq .

# Extract token
TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r ".data.register.token")

# Test 2: Me query (authenticated)
ME_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"{ me { id email createdAt } }"}' \
  http://localhost:8080/graphql)

echo "=== Me Query Test ==="
echo "$ME_RESPONSE" | jq .

# Test 3: Update Profile
PROFILE_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"mutation { updateProfile(input: { displayName: \"Test User\", aboutMe: \"GraphQL Testing\", locationCity: \"TestCity\" }) { displayName aboutMe locationCity isComplete } }"}' \
  http://localhost:8080/graphql)

echo "=== Update Profile Test ==="
echo "$PROFILE_RESPONSE" | jq .

# Test 4: My Profile query
MY_PROFILE_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"query":"{ myProfile { displayName aboutMe locationCity isComplete } }"}' \
  http://localhost:8080/graphql)

echo "=== My Profile Query Test ==="
echo "$MY_PROFILE_RESPONSE" | jq .

# Test 5: Nested User query
USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r ".data.register.user.id")
NESTED_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d "{\"query\":\"{ user(id: \\\"$USER_ID\\\") { id email profile { displayName aboutMe isComplete } } }\"}" \
  http://localhost:8080/graphql)

echo "=== Nested User Query Test ==="
echo "$NESTED_RESPONSE" | jq .

# Test 6: Login with created user
LOGIN_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"query":"mutation { login(email: \"test1@graphql.com\", password: \"password123\") { token user { id email } } }"}' \
  http://localhost:8080/graphql)

echo "=== Login Test ==="
echo "$LOGIN_RESPONSE" | jq .

# Test 7: Logout
LOGOUT_TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r ".data.login.token")
LOGOUT_RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
  -H "Authorization: Bearer $LOGOUT_TOKEN" \
  -d '{"query":"mutation { logout }"}' \
  http://localhost:8080/graphql)

echo "=== Logout Test ==="
echo "$LOGOUT_RESPONSE" | jq .

echo "=== All Tests Completed ==="

