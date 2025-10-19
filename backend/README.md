# Match Me Backend

A Go/PostgreSQL backend for the "Interlink" recommendation platform, matching users based on their analog and digital interests.

---

## Features

- **User Registration:** Secure sign-up with bcrypt-hashed passwords.
- **Login:** JWT-based authentication.
- **Profile Completion:** Users must complete their profile before seeing recommendations.
- **Recommendations:** Returns up to 10 prioritized user IDs based on a weighted matching algorithm.
- **Connections:** Users can see a list of their accepted connections (IDs only).
- **RESTful Endpoints:** Follows project requirements for `/users/{id}`, `/me`, `/recommendations`, `/connections`, etc.
- **Automated Tests:** Includes tests for registration, login, profile, recommendations, and connections.

---

## API Endpoints

See ../docs/API_SPEC.md for full endpoint specification.

---

## Profile JSON Example

```json
{
  "display_name": "User A",
  "about_me": "I love Go!",
  "profile_picture_file": "",
  "location_city": "Testville",
  "location_lat": 10.0,
  "location_lon": 20.0,
  "max_radius_km": 100,
  "analog_passions": ["calligraphy", "knitting"],
  "digital_delights": ["retro gaming"],
  "collaboration_interests": "Looking for a D&D group",
  "favorite_food": "Pizza",
  "favorite_music": "Jazz",
  "other_bio": {},
  "match_preferences": {
    "analog_passions": 5,
    "digital_delights": 3,
    "collaboration_interests": 4,
    "favorite_food": 2,
    "favorite_music": 1,
    "location": 5
  }
}
```

---

## Database Schema

- **users:** `id`, `email`, `password_hash`
- **profiles:** `user_id`, profile fields, `match_preferences` (JSON)
- **connections:** `id`, `user_id`, `target_user_id`, `status` (enum: pending, accepted, dismissed, disconnected), timestamps

---

## Running Tests

```bash
go test
```

- Tests cover registration, login, profile completion, recommendations, and connections.

---

## Project Requirements

- See [copilot-instructions.md](.github/copilot-instructions.md) for full requirements and coding standards.

---

## Notes

- All endpoints require JWT authentication except `/register` and `/login`.
- Email addresses are private and never returned by any endpoint except to the authenticated user.
- Only users with completed profiles can see recommendations or connect.
- All responses for `/users` endpoints include the user ID.
- If a user or profile is not found, or access is not permitted, the API returns HTTP 404.

---

## Next Steps

- Implement endpoints for connection requests (send, accept, dismiss).
- Add chat functionality for connected users.
- Expand RESTful endpoints for `/users/{id}`, `/users/{id}/profile`, `/users/{id}/bio`, `/me`, etc.
- Add more automated and integration tests.
- Ensure accessibility and performance standards are met.

---

## License

MIT
