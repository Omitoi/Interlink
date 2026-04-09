package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strings"
)
// RecommendationResult represents a user recommendation with calculated score
type RecommendationResult struct {
	UserID          int     `json:"user_id"`
	Score           int     `json:"score"`
	ScorePercentage float64 `json:"score_percentage"`
	Distance        float64 `json:"distance,omitempty"`
}

type RecommendationService interface {
	GetRecommendedUserIDs(ctx context.Context, userID int) ([]int, error)
	GetRecommendationsWithScores(ctx context.Context, userID int) ([]RecommendationResult, error)
	IsCurrentlyRecommendable(ctx context.Context, me, targetID int) (bool, error)
	DismissRecommendation(ctx context.Context, userID, dismissedUserID int) error
	CheckProfileComplete(ctx context.Context, userID int) (bool, error)
}

type recommendationService struct {
	repo RecommendationRepository
}

func NewRecommendationService(repo RecommendationRepository) RecommendationService {
	return &recommendationService{repo: repo}
}

func (s *recommendationService) CheckProfileComplete(ctx context.Context, userID int) (bool, error) {
	return s.repo.CheckProfileComplete(ctx, userID)
}

func (s *recommendationService) DismissRecommendation(ctx context.Context, userID, dismissedUserID int) error {
	return s.repo.InsertDismissal(ctx, userID, dismissedUserID)
}

func (s *recommendationService) IsCurrentlyRecommendable(ctx context.Context, me, targetID int) (bool, error) {
	recs, err := s.GetRecommendedUserIDs(ctx, me)
	if err != nil {
		return false, err
	}
	for _, id := range recs {
		if id == targetID {
			return true, nil
		}
	}
	return false, nil
}

// Wrapper for backward compatibility with connections.go
func isCurrentlyRecommendable(ctx context.Context, db *sql.DB, me, targetID int) (bool, error) {
	repo := NewRecommendationRepository(db)
	svc := NewRecommendationService(repo)
	return svc.IsCurrentlyRecommendable(ctx, me, targetID)
}

// Wrapper for backward compatibility with users_profiles.go
func getRecommendedUserIDs(ctx context.Context, db *sql.DB, userID int) ([]int, error) {
	repo := NewRecommendationRepository(db)
	svc := NewRecommendationService(repo)
	return svc.GetRecommendedUserIDs(ctx, userID)
}

func (s *recommendationService) GetRecommendedUserIDs(ctx context.Context, userID int) ([]int, error) {
	results, err := s.GetRecommendationsWithScores(ctx, userID)
	if err != nil {
		return nil, err
	}

	var recommendations []int
	for _, result := range results {
		recommendations = append(recommendations, result.UserID)
	}
	return recommendations, nil
}

func (s *recommendationService) GetRecommendationsWithScores(ctx context.Context, userID int) ([]RecommendationResult, error) {
	userProfile, analogPassions, digitalDelights, matchPrefsRaw, err := s.repo.GetUserProfileData(ctx, userID)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(analogPassions, &userProfile.AnalogPassions)
	json.Unmarshal(digitalDelights, &userProfile.DigitalDelights)
	json.Unmarshal(matchPrefsRaw, &userProfile.MatchPreferences)

	rows, err := s.repo.GetCandidateProfiles(ctx, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidateScore struct {
		UserID   int
		Score    int
		Distance float64
	}
	var candidates []candidateScore

	maxPossibleScore := 0
	for _, weight := range userProfile.MatchPreferences {
		maxPossibleScore += weight
	}
	if maxPossibleScore == 0 {
		maxPossibleScore = 1 // Avoid division by zero
	}

	for rows.Next() {
		var c Profile
		var analog, digital []byte
		err := rows.Scan(&c.UserID, &analog, &digital, &c.CrossPollination, &c.FavoriteFood, &c.FavoriteMusic, &c.LocationLat, &c.LocationLon)
		if err != nil {
			continue
		}
		json.Unmarshal(analog, &c.AnalogPassions)
		json.Unmarshal(digital, &c.DigitalDelights)

		distance := haversine(userProfile.LocationLat, userProfile.LocationLon, c.LocationLat, c.LocationLon)

		if userProfile.MaxRadiusKm > 0 && distance > float64(userProfile.MaxRadiusKm) {
			continue
		}

		score := 0

		analogScore := calculateInterestScore(userProfile.AnalogPassions, c.AnalogPassions)
		score += analogScore * userProfile.MatchPreferences["analog_passions"] / 3

		digitalScore := calculateInterestScore(userProfile.DigitalDelights, c.DigitalDelights)
		score += digitalScore * userProfile.MatchPreferences["digital_delights"] / 3

		crossScore := crossPollinationScore(userProfile.CrossPollination, c.CrossPollination)
		score += crossScore * userProfile.MatchPreferences["collaboration_interests"] / 15

		foodScore := calculateFoodScore(userProfile.FavoriteFood, c.FavoriteFood)
		score += foodScore * userProfile.MatchPreferences["favorite_food"] / 10

		musicScore := calculateMusicScore(userProfile.FavoriteMusic, c.FavoriteMusic)
		score += musicScore * userProfile.MatchPreferences["favorite_music"] / 10

		if userProfile.MatchPreferences["location"] > 0 {
			locationScore := calculateLocationScore(
				userProfile.LocationLat, userProfile.LocationLon,
				c.LocationLat, c.LocationLon,
				userProfile.MaxRadiusKm, userProfile.MatchPreferences["location"])
			score += locationScore
		}

		percentage := (float64(score) / float64(maxPossibleScore)) * 100
		if percentage > 100 {
			percentage = 100
		}

		if percentage >= 25.0 {
			candidates = append(candidates, candidateScore{UserID: c.UserID, Score: score, Distance: distance})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	var results []RecommendationResult
	for i := 0; i < len(candidates) && i < 10; i++ {
		candidate := candidates[i]
		percentage := (float64(candidate.Score) / float64(maxPossibleScore)) * 100
		if percentage > 100 {
			percentage = 100
		}

		result := RecommendationResult{
			UserID:          candidate.UserID,
			Score:           candidate.Score,
			ScorePercentage: percentage,
		}
		if candidate.Distance > 0 {
			result.Distance = candidate.Distance
		}
		results = append(results, result)
	}
	return results, nil
}

// -----------------------------------------------------------------------------
// Scoring algorithms migrated from helper.go
// -----------------------------------------------------------------------------

func calculateInterestScore(userInterests, candidateInterests []string) int {
	if len(userInterests) == 0 || len(candidateInterests) == 0 {
		return 0
	}
	userSet := make(map[string]bool)
	for _, interest := range userInterests {
		userSet[strings.ToLower(interest)] = true
	}
	exactMatches := 0
	partialMatches := 0
	for _, interest := range candidateInterests {
		if userSet[strings.ToLower(interest)] {
			exactMatches++
		}
	}
	semanticGroups := map[string][]string{
		"music":   {"music", "singing", "piano", "guitar", "drums", "composition", "recording"},
		"visual":  {"art", "painting", "drawing", "photography", "design", "graphics"},
		"tech":    {"programming", "coding", "software", "hardware", "electronics", "robotics"},
		"crafts":  {"knitting", "sewing", "woodworking", "pottery", "jewelry", "crafting"},
		"games":   {"gaming", "boardgames", "videogames", "rpg", "strategy", "puzzle"},
		"outdoor": {"hiking", "cycling", "running", "camping", "climbing", "nature"},
		"food":    {"cooking", "baking", "brewing", "wine", "coffee", "culinary"},
		"fitness": {"yoga", "martial arts", "gym", "sports", "dance", "fitness"},
	}
	for _, userInterest := range userInterests {
		userLower := strings.ToLower(userInterest)
		for _, candidateInterest := range candidateInterests {
			candidateLower := strings.ToLower(candidateInterest)
			if userLower == candidateLower {
				continue
			}
			for _, group := range semanticGroups {
				userInGroup := false
				candidateInGroup := false
				for _, groupWord := range group {
					if strings.Contains(userLower, groupWord) {
						userInGroup = true
					}
					if strings.Contains(candidateLower, groupWord) {
						candidateInGroup = true
					}
				}
				if userInGroup && candidateInGroup {
					partialMatches++
					break
				}
			}
		}
	}
	score := exactMatches*3 + partialMatches*1
	totalInterests := len(userInterests) + len(candidateInterests)
	if totalInterests > 0 {
		overlapRatio := float64(exactMatches*2) / float64(totalInterests)
		if overlapRatio > 0.5 {
			score += 5
		}
	}
	return score
}

func crossPollinationScore(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	exactKeywords := []string{"d&d", "dungeons and dragons", "knitting", "blacksmithing", "discord",
		"teaching", "learning", "collaborative", "group", "team", "workshop", "meetup"}
	score := 0
	for _, kw := range exactKeywords {
		if strings.Contains(a, kw) && strings.Contains(b, kw) {
			score += 15
		}
	}
	complementaryPairs := map[string][]string{
		"teach":  {"learn", "student", "beginner"},
		"mentor": {"mentee", "guidance", "help"},
		"code":   {"programming", "development", "software"},
		"design": {"ui", "ux", "graphic", "visual"},
		"music":  {"band", "jam", "collaborate", "duet"},
		"art":    {"paint", "draw", "create", "studio"},
		"craft":  {"handmade", "diy", "workshop", "build"},
		"gaming": {"multiplayer", "coop", "guild", "team"},
	}
	for primary, related := range complementaryPairs {
		if strings.Contains(a, primary) {
			for _, rel := range related {
				if strings.Contains(b, rel) {
					score += 10
				}
			}
		}
		if strings.Contains(b, primary) {
			for _, rel := range related {
				if strings.Contains(a, rel) {
					score += 10
				}
			}
		}
	}
	categories := map[string][]string{
		"creative":    {"art", "design", "music", "writing", "craft", "creative"},
		"technical":   {"code", "programming", "tech", "computer", "digital"},
		"social":      {"group", "team", "community", "meetup", "social"},
		"educational": {"teach", "learn", "study", "workshop", "class"},
		"gaming":      {"game", "gaming", "play", "rpg", "board"},
	}
	for _, words := range categories {
		aMatches := 0
		bMatches := 0
		for _, word := range words {
			if strings.Contains(a, word) {
				aMatches++
			}
			if strings.Contains(b, word) {
				bMatches++
			}
		}
		if aMatches > 0 && bMatches > 0 {
			score += 5
		}
	}
	return score
}

func calculateLocationScore(userLat, userLon, candidateLat, candidateLon float64, maxRadiusKm int, locationWeight int) int {
	distance := haversine(userLat, userLon, candidateLat, candidateLon)
	if maxRadiusKm > 0 && distance > float64(maxRadiusKm) {
		return 0
	}
	if distance == 0 {
		return locationWeight
	}
	if maxRadiusKm == 0 {
		return locationWeight / 2
	}
	proximityRatio := 1.0 - (distance / float64(maxRadiusKm))
	proximityScore := int(proximityRatio * float64(locationWeight))
	bonus := 0
	if distance <= 5 {
		bonus = 5
	} else if distance <= 15 {
		bonus = 2
	}
	if locationWeight == 0 && bonus > 0 {
		return bonus
	}
	return proximityScore + bonus
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371
	dLat := (lat2 - lat1) * (math.Pi / 180)
	dLon := (lon2 - lon1) * (math.Pi / 180)
	lat1 = lat1 * (math.Pi / 180)
	lat2 = lat2 * (math.Pi / 180)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

func calculateFoodScore(userFood, candidateFood string) int {
	if userFood == "" || candidateFood == "" {
		return 0
	}
	if strings.EqualFold(userFood, candidateFood) {
		return 10
	}
	userLower := strings.ToLower(userFood)
	candidateLower := strings.ToLower(candidateFood)
	cuisineGroups := map[string][]string{
		"asian":    {"chinese", "japanese", "thai", "korean", "vietnamese", "indian", "asian"},
		"european": {"italian", "french", "german", "spanish", "greek", "european"},
		"american": {"american", "mexican", "bbq", "burger", "pizza"},
		"healthy":  {"vegan", "vegetarian", "organic", "salad", "healthy"},
		"comfort":  {"comfort", "hearty", "traditional", "home-cooked"},
	}
	for _, group := range cuisineGroups {
		userInGroup := false
		candidateInGroup := false
		for _, cuisine := range group {
			if strings.Contains(userLower, cuisine) {
				userInGroup = true
			}
			if strings.Contains(candidateLower, cuisine) {
				candidateInGroup = true
			}
		}
		if userInGroup && candidateInGroup {
			return 6
		}
	}
	return 0
}

func calculateMusicScore(userMusic, candidateMusic string) int {
	if userMusic == "" || candidateMusic == "" {
		return 0
	}
	if strings.EqualFold(userMusic, candidateMusic) {
		return 10
	}
	userLower := strings.ToLower(userMusic)
	candidateLower := strings.ToLower(candidateMusic)
	genreGroups := map[string][]string{
		"rock":       {"rock", "metal", "punk", "alternative", "grunge"},
		"electronic": {"electronic", "techno", "house", "edm", "ambient", "synth"},
		"pop":        {"pop", "mainstream", "radio", "commercial"},
		"jazz":       {"jazz", "blues", "swing", "bebop"},
		"classical":  {"classical", "orchestra", "symphony", "opera"},
		"hip-hop":    {"hip-hop", "rap", "urban", "r&b"},
		"indie":      {"indie", "independent", "alternative", "underground"},
		"world":      {"world", "folk", "traditional", "ethnic"},
	}
	for _, group := range genreGroups {
		userInGroup := false
		candidateInGroup := false
		for _, genre := range group {
			if strings.Contains(userLower, genre) {
				userInGroup = true
			}
			if strings.Contains(candidateLower, genre) {
				candidateInGroup = true
			}
		}
		if userInGroup && candidateInGroup {
			return 6
		}
	}
	return 0
}
