# GraphQL API Documentation

## Overview

The Match-Me application provides a comprehensive GraphQL API alongside the existing REST API. The GraphQL endpoint offers flexible querying capabilities with strong type safety and real-time features.

## Endpoints

- **GraphQL API**: `http://localhost:8080/graphql`
- **GraphQL Playground** (development): `http://localhost:8080/`

## Authentication

The GraphQL API uses JWT (JSON Web Token) authentication. Include the token in the Authorization header:

```
Authorization: Bearer <your-jwt-token>
```

## Schema Overview

### Types

#### User
```graphql
type User {
  id: ID!
  email: String!
  createdAt: String!
  updatedAt: String!
  lastOnline: String
  profile: Profile
  bio: Bio
}
```

#### Profile
```graphql
type Profile {
  userID: ID!
  displayName: String!
  aboutMe: String
  profilePictureFile: String
  locationCity: String
  locationLat: Float
  locationLon: Float
  maxRadiusKm: Int
  isComplete: Boolean!
  user: User!
}
```

#### Bio
```graphql
type Bio {
  userID: ID!
  analogPassions: [String!]!
  digitalDelights: [String!]!
  collaborationInterests: String
  favoriteFood: String
  favoriteMusic: String
  user: User!
}
```

## Queries

### Public Queries

#### Get User by ID
```graphql
query {
  user(id: "123") {
    id
    email
    profile {
      displayName
      aboutMe
      isComplete
    }
    bio {
      analogPassions
      digitalDelights
    }
  }
}
```

#### Get User Profile
```graphql
query {
  userProfile(id: "123") {
    displayName
    aboutMe
    locationCity
    isComplete
  }
}
```

#### Get User Bio
```graphql
query {
  userBio(id: "123") {
    analogPassions
    digitalDelights
    collaborationInterests
    favoriteFood
    favoriteMusic
  }
}
```

#### Get Recommendations
```graphql
query {
  recommendations {
    id
    email
    profile {
      displayName
      aboutMe
    }
  }
}
```

### Protected Queries (Require Authentication)

#### Get Current User
```graphql
query {
  me {
    id
    email
    createdAt
    updatedAt
    lastOnline
  }
}
```

#### Get My Profile
```graphql
query {
  myProfile {
    displayName
    aboutMe
    locationCity
    maxRadiusKm
    isComplete
  }
}
```

#### Get My Bio
```graphql
query {
  myBio {
    analogPassions
    digitalDelights
    collaborationInterests
    favoriteFood
    favoriteMusic
  }
}
```

## Mutations

### Authentication

#### Register
```graphql
mutation {
  register(email: "user@example.com", password: "securepass123") {
    token
    user {
      id
      email
      createdAt
    }
  }
}
```

#### Login
```graphql
mutation {
  login(email: "user@example.com", password: "securepass123") {
    token
    user {
      id
      email
      lastOnline
    }
  }
}
```

#### Logout
```graphql
mutation {
  logout
}
```

### Profile Management

#### Update Profile
```graphql
mutation {
  updateProfile(input: {
    displayName: "John Doe"
    aboutMe: "Software developer interested in analog hobbies"
    locationCity: "San Francisco"
    maxRadiusKm: 50
  }) {
    displayName
    aboutMe
    locationCity
    maxRadiusKm
    isComplete
  }
}
```

## Input Types

### ProfileInput
```graphql
input ProfileInput {
  displayName: String
  aboutMe: String
  locationCity: String
  locationLat: Float
  locationLon: Float
  maxRadiusKm: Int
}
```

### BioInput
```graphql
input BioInput {
  analogPassions: [String!]
  digitalDelights: [String!]
  collaborationInterests: String
  favoriteFood: String
  favoriteMusic: String
}
```

## Error Handling

GraphQL errors are returned in the standard format:

```json
{
  "errors": [
    {
      "message": "authentication required",
      "path": ["me"]
    }
  ],
  "data": null
}
```

Common error types:
- `"authentication required"` - Missing or invalid JWT token
- `"invalid credentials"` - Wrong email/password combination
- `"email already exists"` - Registration with existing email
- `"user not found"` - Invalid user ID in query
- `"failed to update profile"` - Database error during profile update

## Example Workflows

### 1. User Registration and Profile Setup

```graphql
# Step 1: Register
mutation {
  register(email: "newuser@example.com", password: "securepass123") {
    token
    user {
      id
      email
    }
  }
}

# Step 2: Update Profile (using token from step 1)
mutation {
  updateProfile(input: {
    displayName: "New User"
    aboutMe: "Excited to connect with like-minded people"
    locationCity: "New York"
  }) {
    displayName
    aboutMe
    isComplete
  }
}

# Step 3: Query complete profile
query {
  me {
    id
    email
    profile {
      displayName
      aboutMe
      isComplete
    }
  }
}
```

### 2. Exploring Other Users

```graphql
# Get recommendations
query {
  recommendations {
    id
    email
    profile {
      displayName
      aboutMe
      locationCity
    }
    bio {
      analogPassions
      digitalDelights
    }
  }
}

# Get specific user details
query {
  user(id: "456") {
    email
    profile {
      displayName
      aboutMe
      locationCity
    }
    bio {
      analogPassions
      digitalDelights
      collaborationInterests
    }
  }
}
```

## Performance Features

### Nested Field Resolvers
The API efficiently handles nested queries using field resolvers, preventing N+1 query problems:

```graphql
query {
  user(id: "123") {
    email
    profile {
      displayName
      user {
        id
        email
      }
    }
  }
}
```

### Selective Data Fetching
GraphQL allows clients to request only the data they need:

```graphql
# Minimal user data
query {
  user(id: "123") {
    id
    email
  }
}

# Complete user data
query {
  user(id: "123") {
    id
    email
    createdAt
    updatedAt
    lastOnline
    profile {
      displayName
      aboutMe
      locationCity
      locationLat
      locationLon
      maxRadiusKm
      isComplete
    }
    bio {
      analogPassions
      digitalDelights
      collaborationInterests
      favoriteFood
      favoriteMusic
    }
  }
}
```

## Security Features

1. **JWT Authentication**: All protected endpoints require valid JWT tokens
2. **Password Hashing**: Passwords are hashed using bcrypt with salt
3. **Private Data Protection**: Email addresses are only visible to the authenticated user
4. **Input Validation**: All inputs are validated and sanitized
5. **SQL Injection Prevention**: All database queries use parameterized statements

## Development Tools

### GraphQL Playground
Access the interactive GraphQL playground at `http://localhost:8080/` (development mode only).

Features:
- Interactive query editor with syntax highlighting
- Auto-completion and schema introspection
- Built-in documentation browser
- Query history and variables support

### Testing
Run the comprehensive test suite:

```bash
# Integration tests
go test -v -run TestGraphQLIntegration

# Manual testing script
./test_manual.sh
```

## Future Enhancements

Planned features for future releases:
- [ ] GraphQL Subscriptions for real-time features
- [ ] DataLoader implementation for optimized nested queries
- [ ] File upload support for profile pictures
- [ ] Connection and chat management
- [ ] Advanced filtering and pagination
- [ ] Rate limiting and query complexity analysis

## Compatibility

The GraphQL API is designed to coexist with the existing REST API:
- Both APIs share the same database and authentication system
- JWT tokens are compatible between REST and GraphQL endpoints
- Data models are consistent across both APIs
- Clients can use REST and GraphQL endpoints interchangeably