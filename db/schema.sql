CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL CHECK (email <> ''),
    password_hash VARCHAR(60) NOT NULL CHECK (char_length(password_hash) = 60),
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    last_online TIMESTAMPTZ
);

CREATE TABLE profiles (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    display_name VARCHAR(100) NOT NULL CHECK (display_name <> ''),
    about_me TEXT,
    profile_picture_file VARCHAR(255),
    location_city VARCHAR(100),
    location_lat DOUBLE PRECISION CHECK (location_lat BETWEEN -90 AND 90),
    location_lon DOUBLE PRECISION CHECK (location_lon BETWEEN -180 AND 180),
    max_radius_km INTEGER CHECK (max_radius_km > 0),
    analog_passions JSONB,
    digital_delights JSONB,
    collaboration_interests TEXT,
    favorite_food VARCHAR(100),
    favorite_music VARCHAR(100),
    other_bio JSONB,
    match_preferences JSONB,
    is_complete BOOLEAN DEFAULT FALSE NOT NULL
);

CREATE TYPE connection_status AS ENUM ('pending', 'accepted', 'dismissed', 'disconnected');

CREATE TABLE connections (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    target_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status connection_status NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    CHECK (user_id <> target_user_id),
    UNIQUE (user_id, target_user_id)
);

CREATE TABLE dismissed_recommendations (
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dismissed_user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dismissed_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    PRIMARY KEY (user_id, dismissed_user_id),
    CHECK (user_id <> dismissed_user_id)
);

CREATE TABLE chats (
    id SERIAL PRIMARY KEY,
    user1_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user2_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_message_at TIMESTAMPTZ,
    unread_for_user1 BOOLEAN DEFAULT FALSE NOT NULL,
    unread_for_user2 BOOLEAN DEFAULT FALSE NOT NULL,
    UNIQUE (user1_id, user2_id),
    CHECK (user1_id <> user2_id)
);

CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    chat_id INTEGER NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    sender_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL CHECK (char_length(content) > 0),
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL,
    is_read BOOLEAN DEFAULT FALSE NOT NULL
);

CREATE INDEX idx_profiles_location ON profiles (location_lat, location_lon);
CREATE INDEX idx_connections_user ON connections (user_id);
CREATE INDEX idx_connections_target ON connections (target_user_id);
CREATE INDEX idx_connections_status ON connections (status);
CREATE INDEX idx_messages_chat_created ON messages (chat_id, created_at DESC);
CREATE INDEX idx_profiles_match_preferences ON profiles USING gin (match_preferences);
