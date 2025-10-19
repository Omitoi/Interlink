# Match-Me API Specification

Version: v0.1 (2025-08-20)

Status: Draft

## Overview

Unified REST + WebSocket contract for the Match-Me ("Interlink") platform. All authenticated endpoints require a Bearer JWT in the `Authorization` header unless specified.

```text
Authorization: Bearer <token>
```

If authentication fails: `401 {"error":"unauthorized"}`.
Profiles that are not visible due to permissions MUST return 404 (to avoid information leakage), identical to a genuinely missing record.

Error envelope (all non-2xx):

```json
{"error":"<lowercase terse message>"}
```

## Conventions

- Timestamps: RFC3339 UTC.
- Arrays: empty array `[]` instead of `null`.
- Omitted optional fields are `null` or absent (decide & be consistent; current leaning: omit).
- Numeric IDs: integer.
- Rate limiting (future): 429 with standard envelope.

## Authentication & Session

### POST /register

Request: `{"email":"user@example.com","password":"yourpassword"}`

Responses:

- 201 `{ "id": <int> }`
- 400 invalid input
- 409 duplicate

### POST /login

Request: `{"email":"user@example.com","password":"yourpassword"}`

Responses:

- 200 `{ "token": "<jwt>", "id": <int> }`
- 401 invalid credentials

### POST /logout *(optional/stateless)*

Client discards token. (May be omitted from initial build.)

- 204

## Current User Shortcuts

### GET /me

Returns the same minimal summary structure as /users/{id} for the authenticated user.

```json
{ "id": 1, "display_name": "Alice", "profile_picture": null }
```

### GET /me/profile

Full profile basics for self (adds completion & location metadata).

```json
{
  "id": int,
  "display_name": string,
  "about_me": string,
  "profile_picture": string|null,
  "location_city": string|null,
  "is_complete": bool
}
```

### GET /me/bio

Recommendation-driving bio facets.

```json
{
  "id": int,
  "analog_passions": [string],
  "digital_delights": [string],
  "seeking": [string],
  "interests": [string],
  "preferred_location_radius_km": int|null
}
```

### POST or PATCH /me/profile/complete

Create or update (idempotent upsert) the profile and mark it complete.

Allowed fields: `display_name, about_me, location_city, profile_picture_file, location_lat, location_lon, max_radius_km, analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music, other_bio, match_preferences`

Response (200): `{ "status": "ok" }`

### GET /me/bio (self)

See bio facets below. (Write operations on bio facets are currently combined with profile completion at `/me/profile/complete`).

## Permission-Gated User Resource Views

These endpoints do NOT expose public data. Access is only granted if (a) the target user is in current recommendations, (b) there is a pending or accepted connection between requester and target, or (c) they are already connected. Otherwise they MUST return HTTP404 (indistinguishable from non-existent user) to prevent enumeration.

### GET /users/{id}

`200 { "id": int, "display_name": string, "profile_picture": string|null }`

### GET /users/{id}/profile

`200 { "id": int, "display_name": string, "about_me": string, "profile_picture": string|null, "location_city": string|null }`

### GET /users/{id}/bio

Returns biographical facets used for recommendation scoring. Field names mirror stored columns.

```json
{
  "id": 42,
  "analog_passions": ["calligraphy"],
  "digital_delights": ["retro gaming"],
  "seeking": ["collaboration_interests"],
  "interests": ["favorite_music"]
}
```

Note: In the current implementation `seeking` is derived from `collaboration_interests` and `interests` reuses `favorite_music` until a richer structure is added.

## Recommendations

### GET /recommendations

Preconditions: profile complete. If incomplete → `403 {"error":"incomplete_profile"}`.

`200 { "recommendations": [int] }` (≤ 10 strongest first)

### (Planned) POST /recommendations/{id}/dismiss

Planned endpoint (not yet implemented) to permanently remove a recommendation for the requester.

Intended response: `201 { "dismissed": true }` or 404 if not visible.

## Connections

### GET /connections

`200 { "connections": [int] }`

### GET /connections/requests

`200 { "incoming": [int], "outgoing": [int] }`

### POST /connections/requests

Request: `{ "target_id": int }`

Responses:

- 201 `{ "request_id": int, "target_id": int }`
- 409 already pending/connected

### POST /connections/requests/{id}/accept

`200 { "connected": true, "user_id": int, "target_id": int }`

### POST /connections/requests/{id}/reject

`200 { "rejected": true }`

### DELETE /connections/{user_id}

Disconnect. `204`

## Chat

Requires connection.

### GET /chats

```json
{
  "chats": [
    {"chat_id": int, "user_id": int, "other_user_id": int, "last_message_preview": string, "last_message_at": "RFC3339", "unread_count": int}
  ]
}
```

### POST /chats

Request: `{ "other_user_id": int }`

`201 { "chat_id": int }`

### GET /chats/{chat_id}/messages?limit=50&before=<msg_id>

```json
{
 "chat_id": int,
 "messages": [
   {"id": int,"sender_id": int,"content": string,"created_at": "RFC3339"}
 ],
 "next_cursor": string|null
}
```

### POST /chats/{chat_id}/messages

Request: `{ "content": string }`

`201 { "id": int, "chat_id": int, "sender_id": int, "content": string, "created_at": "RFC3339" }`

## WebSocket /ws

Auth: query `?token=` or header.

Events:

- Client -> `{ "type": "typing", "chat_id": int }`
- Client -> `{ "type": "message", "chat_id": int, "content": string }`
- Server -> `{ "type": "message", "chat_id": int, "payload": { <message> } }`
- Server -> `{ "type": "typing", "chat_id": int, "user_id": int }`
- Server -> `{ "type": "chat_unread", "chat_id": int, "unread_count": int }`

Heartbeat: ping/pong every 30s.

## Images

### POST /me/profile/picture

Multipart upload. `201 { "profile_picture": "url" }`

### DELETE /me/profile/picture

`204`

## Location

### PATCH /me/location

Request: `{ "latitude": float, "longitude": float }`

`200 { "ok": true }`

## Admin / Seed (dev only)

### POST /admin/seed

Request: `{ "count": 100 }`

`202 { "seeded": int }`

## Validation Constraints (Baseline)

- display_name: 2-40 chars
- about_me: ≤ 1000 chars
- list entries: 1-40 chars, max 15 per list
- password: ≥ 8 chars
- chat message: 1-2000 chars
- image size: ≤ 2MB

## Recommendation Scoring (Internal Outline)

Weighted components (example weights):

- Shared analog passions (0.25)
- Complementary analog↔digital cross-pollination (0.20)
- Shared digital delights (0.15)
- Seeking overlap (0.25)
- Location feasibility (0.10 gate + scale)
- Novelty penalty (0.05)

Return top ≤10 non-dismissed.

## Security Notes

- 404 masking for unauthorized profile/bio endpoints.
- Rate limit login & message send.
- JWT exp (e.g. 15m) — refresh token TBD.
- Validate chat access vs connections.

## Open Items

- [ ] Refresh token strategy
- [ ] Cursor format (base64?)
- [ ] Reversible dismiss?
- [ ] Soft delete semantics for disconnect
- [ ] CDN / image storage decision

## Schema Additions (Planned / Ensure)

```sql
users(id,email,password_hash,created_at)
profiles(user_id PK, display_name, about_me, profile_picture_file, location_city, is_complete,
         latitude, longitude, preferred_radius_km,
         analog_passions TEXT[], digital_delights TEXT[], seeking TEXT[], interests TEXT[])
connections(id,user_id,target_user_id,status,created_at, UNIQUE(user_id,target_user_id))
connection_requests(id,requester_id,target_id,status,created_at)
dismissed_recommendations(user_id,dismissed_user_id,created_at, UNIQUE(user_id,dismissed_user_id))
chats(id,user_a_id,user_b_id,created_at, UNIQUE(user_a_id,user_b_id))
messages(id,chat_id,sender_id,content,created_at, INDEX(chat_id,created_at DESC))
```

## Changelog

- v0.1 Initial draft.
