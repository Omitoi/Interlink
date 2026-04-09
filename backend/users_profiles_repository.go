package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type UserProfileData struct {
	DisplayName     string
	AboutMe         string
	ProfilePicture  sql.NullString
	LocationCity    string
	LocationLat     sql.NullFloat64
	LocationLon     sql.NullFloat64
	MaxRadiusKm     sql.NullInt64
	AnalogPassions  json.RawMessage
	DigitalDelights json.RawMessage
	Collaboration   string
	FavoriteFood    string
	FavoriteMusic   string
	OtherBio        json.RawMessage
	MatchPrefs      json.RawMessage
	IsComplete      sql.NullBool
}

type ProfileRequest struct {
	DisplayName      string          `json:"display_name"`
	AboutMe          string          `json:"about_me"`
	ProfilePicture   string          `json:"profile_picture_file"`
	LocationCity     string          `json:"location_city"`
	LocationLat      float64         `json:"location_lat"`
	LocationLon      float64         `json:"location_lon"`
	MaxRadiusKm      int             `json:"max_radius_km"`
	AnalogPassions   json.RawMessage `json:"analog_passions"`
	DigitalDelights  json.RawMessage `json:"digital_delights"`
	CrossPollination string          `json:"collaboration_interests"`
	FavoriteFood     string          `json:"favorite_food"`
	FavoriteMusic    string          `json:"favorite_music"`
	OtherBio         json.RawMessage `json:"other_bio"`
	MatchPreferences json.RawMessage `json:"match_preferences"`
}

type UserProfileRepository interface {
	GetBasicUserInfo(ctx context.Context, userID int) (string, string, error)
	GetProfileInfo(ctx context.Context, userID int) (string, string, string, error)
	GetProfileLocation(ctx context.Context, userID int) (sql.NullFloat64, sql.NullFloat64, error)
	GetProfileBio(ctx context.Context, userID int) (json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, error)
	GetFullProfile(ctx context.Context, userID int) (UserProfileData, error)
	GetMeBio(ctx context.Context, userID int) (json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, error)
	UpsertProfile(ctx context.Context, userID int, req ProfileRequest) error
	CanViewUser(ctx context.Context, viewerID, targetID int) bool
}

type sqlUserProfileRepo struct {
	db *sql.DB
}

func NewUserProfileRepository(db *sql.DB) UserProfileRepository {
	return &sqlUserProfileRepo{db: db}
}

func (r *sqlUserProfileRepo) GetBasicUserInfo(ctx context.Context, userID int) (string, string, error) {
	var displayName, profilePicture string
	err := r.db.QueryRowContext(ctx, `
        SELECT
            COALESCE(p.display_name, 'User ' || u.id::text) AS display_name,
            COALESCE(p.profile_picture_file, 'avatar_placeholder.png') AS profile_picture_file
        FROM users u
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE u.id = $1
    `, userID).Scan(&displayName, &profilePicture)
	return displayName, profilePicture, err
}

func (r *sqlUserProfileRepo) GetProfileInfo(ctx context.Context, userID int) (string, string, string, error) {
	var aboutMe, displayName string
	var profilePictureSQL sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT about_me, display_name, profile_picture_file FROM profiles WHERE user_id = $1",
		userID,
	).Scan(&aboutMe, &displayName, &profilePictureSQL)

	profilePicture := "avatar_placeholder.png"
	if profilePictureSQL.Valid && strings.TrimSpace(profilePictureSQL.String) != "" {
		profilePicture = profilePictureSQL.String
	}
	return aboutMe, displayName, profilePicture, err
}

func (r *sqlUserProfileRepo) GetProfileLocation(ctx context.Context, userID int) (sql.NullFloat64, sql.NullFloat64, error) {
	var lat, lon sql.NullFloat64
	err := r.db.QueryRowContext(ctx, `
		SELECT location_lat, location_lon 
		FROM profiles 
		WHERE user_id = $1
	`, userID).Scan(&lat, &lon)
	return lat, lon, err
}

func (r *sqlUserProfileRepo) GetProfileBio(ctx context.Context, userID int) (json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, error) {
	var analog, digital, collaborationInterests, favoriteFood, favoriteMusic json.RawMessage
	err := r.db.QueryRowContext(ctx, `SELECT analog_passions, digital_delights, to_jsonb(collaboration_interests), to_jsonb(favorite_food), to_jsonb(favorite_music) FROM profiles WHERE user_id = $1`, userID).Scan(&analog, &digital, &collaborationInterests, &favoriteFood, &favoriteMusic)
	return analog, digital, collaborationInterests, favoriteFood, favoriteMusic, err
}

func (r *sqlUserProfileRepo) GetFullProfile(ctx context.Context, userID int) (UserProfileData, error) {
	var data UserProfileData
	err := r.db.QueryRowContext(ctx, `
		SELECT display_name, about_me, profile_picture_file, location_city, location_lat, location_lon, 
		       max_radius_km, analog_passions, digital_delights, collaboration_interests, favorite_food, 
		       favorite_music, other_bio, match_preferences, is_complete
		FROM profiles WHERE user_id = $1
	`, userID).Scan(
		&data.DisplayName, &data.AboutMe, &data.ProfilePicture, &data.LocationCity, &data.LocationLat, &data.LocationLon,
		&data.MaxRadiusKm, &data.AnalogPassions, &data.DigitalDelights, &data.Collaboration, &data.FavoriteFood,
		&data.FavoriteMusic, &data.OtherBio, &data.MatchPrefs, &data.IsComplete,
	)
	return data, err
}

func (r *sqlUserProfileRepo) GetMeBio(ctx context.Context, userID int) (json.RawMessage, json.RawMessage, json.RawMessage, json.RawMessage, error) {
	var analog, digital, seeking, interests json.RawMessage
	err := r.db.QueryRowContext(ctx, `SELECT analog_passions, digital_delights, to_jsonb(collaboration_interests), to_jsonb(favorite_music) FROM profiles WHERE user_id = $1`, userID).Scan(&analog, &digital, &seeking, &interests)
	return analog, digital, seeking, interests, err
}

func (r *sqlUserProfileRepo) UpsertProfile(ctx context.Context, userID int, req ProfileRequest) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO profiles (
			user_id, display_name, about_me, location_city, location_lat, location_lon, max_radius_km,
			analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music, other_bio, match_preferences, is_complete
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, TRUE
		)
		ON CONFLICT (user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			about_me = EXCLUDED.about_me,
			location_city = EXCLUDED.location_city,
			location_lat = EXCLUDED.location_lat,
			location_lon = EXCLUDED.location_lon,
			max_radius_km = EXCLUDED.max_radius_km,
			analog_passions = EXCLUDED.analog_passions,
			digital_delights = EXCLUDED.digital_delights,
			collaboration_interests = EXCLUDED.collaboration_interests,
			favorite_food = EXCLUDED.favorite_food,
			favorite_music = EXCLUDED.favorite_music,
			other_bio = EXCLUDED.other_bio,
			match_preferences = EXCLUDED.match_preferences,
			is_complete = TRUE
	`,
		userID, req.DisplayName, req.AboutMe, req.LocationCity, req.LocationLat, req.LocationLon, req.MaxRadiusKm,
		req.AnalogPassions, req.DigitalDelights, req.CrossPollination, req.FavoriteFood, req.FavoriteMusic, req.OtherBio, req.MatchPreferences,
	)
	return err
}

func (r *sqlUserProfileRepo) CanViewUser(ctx context.Context, viewerID, targetID int) bool {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM connections
		WHERE ((user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1))
		AND status IN ('accepted', 'pending')
	`, viewerID, targetID).Scan(&count)
	return err == nil && count > 0
}
