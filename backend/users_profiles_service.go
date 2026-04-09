package main

import (
	"context"
	"database/sql"
	"encoding/json"

	"golang.org/x/sync/errgroup"
)

type UserProfileService interface {
	GetBasicUserInfoWithPresence(ctx context.Context, userID int) (map[string]interface{}, error)
	GetTargetProfile(ctx context.Context, requesterID, targetID int) (map[string]interface{}, error)
	GetTargetBio(ctx context.Context, requesterID, targetID int) (map[string]interface{}, error)
	UpsertProfile(ctx context.Context, userID int, req ProfileRequest) error
	GetMeBio(ctx context.Context, userID int) (map[string]interface{}, error)
	GetMeFullProfile(ctx context.Context, userID int) (map[string]interface{}, error)
	GetMeBasicProfile(ctx context.Context, userID int) (map[string]interface{}, error)
}

type userProfileService struct {
	repo UserProfileRepository
	db   *sql.DB
}

func NewUserProfileService(repo UserProfileRepository, db *sql.DB) UserProfileService {
	return &userProfileService{repo: repo, db: db}
}

func (s *userProfileService) GetBasicUserInfoWithPresence(ctx context.Context, userID int) (map[string]interface{}, error) {
	displayName, profilePicture, err := s.repo.GetBasicUserInfo(ctx, userID)
	if err != nil {
		return nil, ErrNotFound
	}

	onlineDB, _ := isOnlineNow(ctx, s.db, userID)
	return map[string]interface{}{
		"id":              userID,
		"display_name":    displayName,
		"profile_picture": profilePicture,
		"is_online":       onlineDB,
	}, nil
}

func (s *userProfileService) GetTargetProfile(ctx context.Context, requesterID, targetID int) (map[string]interface{}, error) {
	allowed := s.repo.CanViewUser(ctx, requesterID, targetID)
	if !allowed {
		recs, err := getRecommendedUserIDs(ctx, s.db, requesterID)
		if err == nil {
			for _, id := range recs {
				if id == targetID {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		return nil, ErrNotFound
	}

	g, gCtx := errgroup.WithContext(ctx)

	var (
		aboutMe, displayName, profilePicture string
		locationLat, locationLon             sql.NullFloat64
		onlineDB                             bool
	)

	g.Go(func() error {
		var err error
		aboutMe, displayName, profilePicture, err = s.repo.GetProfileInfo(gCtx, targetID)
		if err != nil {
			return err
		}
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			return nil
		}
	})

	g.Go(func() error {
		lat, lon, err := s.repo.GetProfileLocation(gCtx, targetID)
		if err == nil {
			locationLat = lat
			locationLon = lon
		}
		return nil
	})

	g.Go(func() error {
		onlineDB, _ = isOnlineNow(gCtx, s.db, targetID)
		select {
		case <-gCtx.Done():
			return gCtx.Err()
		default:
			return nil
		}
	})

	if err := g.Wait(); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	resp := map[string]interface{}{
		"id":              targetID,
		"display_name":    displayName,
		"profile_picture": profilePicture,
		"about_me":        aboutMe,
		"is_online":       onlineDB,
	}

	if locationLat.Valid {
		resp["location_lat"] = locationLat.Float64
	}
	if locationLon.Valid {
		resp["location_lon"] = locationLon.Float64
	}
	return resp, nil
}

func (s *userProfileService) GetTargetBio(ctx context.Context, requesterID, targetID int) (map[string]interface{}, error) {
	allowed := s.repo.CanViewUser(ctx, requesterID, targetID)
	if !allowed {
		recs, err := getRecommendedUserIDs(ctx, s.db, requesterID)
		if err == nil {
			for _, id := range recs {
				if id == targetID {
					allowed = true
					break
				}
			}
		}
	}
	if !allowed {
		return nil, ErrNotFound
	}

	analog, digital, collaborationInterests, favoriteFood, favoriteMusic, err := s.repo.GetProfileBio(ctx, targetID)
	if err != nil {
		return nil, ErrNotFound
	}

	return map[string]interface{}{
		"id":                      targetID,
		"analog_passions":         jsonRawOrArray(analog),
		"digital_delights":        jsonRawOrArray(digital),
		"collaboration_interests": jsonRawOrArray(collaborationInterests),
		"favorite_food":           jsonRawOrArray(favoriteFood),
		"favorite_music":          jsonRawOrArray(favoriteMusic),
	}, nil
}

func (s *userProfileService) UpsertProfile(ctx context.Context, userID int, req ProfileRequest) error {
	return s.repo.UpsertProfile(ctx, userID, req)
}

func (s *userProfileService) GetMeBio(ctx context.Context, userID int) (map[string]interface{}, error) {
	analog, digital, seeking, interests, err := s.repo.GetMeBio(ctx, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return map[string]interface{}{
		"id":               userID,
		"analog_passions":  jsonRawOrArray(analog),
		"digital_delights": jsonRawOrArray(digital),
		"seeking":          jsonRawOrArray(seeking),
		"interests":        jsonRawOrArray(interests),
	}, nil
}

func (s *userProfileService) GetMeFullProfile(ctx context.Context, userID int) (map[string]interface{}, error) {
	data, err := s.repo.GetFullProfile(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}

	profilePictureValue := ""
	if data.ProfilePicture.Valid {
		profilePictureValue = data.ProfilePicture.String
	}

	response := map[string]interface{}{
		"id":                      userID,
		"display_name":            data.DisplayName,
		"about_me":                data.AboutMe,
		"profile_picture":         profilePictureValue,
		"location_city":           data.LocationCity,
		"collaboration_interests": data.Collaboration,
		"favorite_food":           data.FavoriteFood,
		"favorite_music":          data.FavoriteMusic,
		"is_complete":             data.IsComplete.Bool,
	}

	if data.LocationLat.Valid {
		response["location_lat"] = data.LocationLat.Float64
	}
	if data.LocationLon.Valid {
		response["location_lon"] = data.LocationLon.Float64
	}
	if data.MaxRadiusKm.Valid {
		response["max_radius_km"] = data.MaxRadiusKm.Int64
	}

	if data.AnalogPassions != nil {
		response["analog_passions"] = jsonRawOrArray(data.AnalogPassions)
	}
	if data.DigitalDelights != nil {
		response["digital_delights"] = jsonRawOrArray(data.DigitalDelights)
	}
	if data.OtherBio != nil {
		var parsed interface{}
		if json.Unmarshal(data.OtherBio, &parsed) == nil {
			response["other_bio"] = parsed
		}
	}
	if data.MatchPrefs != nil {
		var parsed interface{}
		if json.Unmarshal(data.MatchPrefs, &parsed) == nil {
			response["match_preferences"] = parsed
		}
	}

	return response, nil
}

func (s *userProfileService) GetMeBasicProfile(ctx context.Context, userID int) (map[string]interface{}, error) {
	displayName, profilePicture, err := s.repo.GetBasicUserInfo(ctx, userID)
	if err != nil {
		return nil, ErrNotFound
	}
	return map[string]interface{}{
		"id":              userID,
		"display_name":    displayName,
		"profile_picture": profilePicture,
	}, nil
}
