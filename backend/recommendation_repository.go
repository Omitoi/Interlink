package main

import (
	"context"
	"database/sql"
)

type RecommendationRepository interface {
	GetUserProfileData(ctx context.Context, userID int) (Profile, []byte, []byte, []byte, error)
	GetCandidateProfiles(ctx context.Context, userID int) (*sql.Rows, error)
	InsertDismissal(ctx context.Context, userID, dismissedUserID int) error
	CheckProfileComplete(ctx context.Context, userID int) (bool, error)
}

type sqlRecommendationRepo struct {
	db *sql.DB
}

func NewRecommendationRepository(db *sql.DB) RecommendationRepository {
	return &sqlRecommendationRepo{db: db}
}

func (r *sqlRecommendationRepo) CheckProfileComplete(ctx context.Context, userID int) (bool, error) {
	var isComplete bool
	err := r.db.QueryRowContext(ctx, "SELECT COALESCE(is_complete, FALSE) FROM profiles WHERE user_id = $1", userID).Scan(&isComplete)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return isComplete, err
}

func (r *sqlRecommendationRepo) GetUserProfileData(ctx context.Context, userID int) (Profile, []byte, []byte, []byte, error) {
	var userProfile Profile
	var analogPassions, digitalDelights, matchPrefsRaw []byte
	err := r.db.QueryRowContext(ctx, `
        SELECT user_id, analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music,
               location_lat, location_lon, max_radius_km, match_preferences
        FROM profiles WHERE user_id = $1
    `, userID).Scan(
		&userProfile.UserID, &analogPassions, &digitalDelights, &userProfile.CrossPollination,
		&userProfile.FavoriteFood, &userProfile.FavoriteMusic,
		&userProfile.LocationLat, &userProfile.LocationLon, &userProfile.MaxRadiusKm, &matchPrefsRaw,
	)
	return userProfile, analogPassions, digitalDelights, matchPrefsRaw, err
}

func (r *sqlRecommendationRepo) GetCandidateProfiles(ctx context.Context, userID int) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, `
        SELECT p.user_id,
               p.analog_passions,
               p.digital_delights,
               p.collaboration_interests,
               p.favorite_food,
               p.favorite_music,
               p.location_lat,
               p.location_lon
        FROM profiles p
        WHERE p.is_complete = TRUE
          AND p.user_id <> $1
          AND NOT EXISTS (
              SELECT 1
              FROM connections c
              WHERE (c.user_id = $1 AND c.target_user_id = p.user_id)
                 OR (c.user_id = p.user_id AND c.target_user_id = $1)
          )
          AND NOT EXISTS (
              SELECT 1
              FROM dismissed_recommendations d
              WHERE d.user_id = $1 AND d.dismissed_user_id = p.user_id
          )
    `, userID)
}

func (r *sqlRecommendationRepo) InsertDismissal(ctx context.Context, userID, dismissedUserID int) error {
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM users JOIN profiles ON users.id = profiles.user_id WHERE users.id = $1 AND profiles.is_complete = TRUE)", dismissedUserID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists || dismissedUserID == userID {
		return ErrNotFound
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO dismissed_recommendations (user_id, dismissed_user_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, userID, dismissedUserID)
	return err
}
