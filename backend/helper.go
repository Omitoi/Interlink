package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"strings"
)

// Enhanced interest matching with weighted scoring
func calculateInterestScore(userInterests, candidateInterests []string) int {
	if len(userInterests) == 0 || len(candidateInterests) == 0 {
		return 0
	}

	// Create frequency maps
	userSet := make(map[string]bool)
	for _, interest := range userInterests {
		userSet[strings.ToLower(interest)] = true
	}

	exactMatches := 0
	partialMatches := 0

	// Count exact matches
	for _, interest := range candidateInterests {
		if userSet[strings.ToLower(interest)] {
			exactMatches++
		}
	}

	// Count partial/semantic matches
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

			// Skip if exact match already counted
			if userLower == candidateLower {
				continue
			}

			// Check semantic similarity
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

	// Calculate score: exact matches worth more than partial
	score := exactMatches*3 + partialMatches*1

	// Bonus for high overlap percentage
	totalInterests := len(userInterests) + len(candidateInterests)
	if totalInterests > 0 {
		overlapRatio := float64(exactMatches*2) / float64(totalInterests)
		if overlapRatio > 0.5 {
			score += 5 // High overlap bonus
		}
	}

	return score
}

// Enhanced cross-pollination matching with scoring
func crossPollinationScore(a, b string) int {
	a = strings.ToLower(a)
	b = strings.ToLower(b)

	// Exact keyword matches (highest score)
	exactKeywords := []string{"d&d", "dungeons and dragons", "knitting", "blacksmithing", "discord",
		"teaching", "learning", "collaborative", "group", "team", "workshop", "meetup"}
	score := 0

	for _, kw := range exactKeywords {
		if strings.Contains(a, kw) && strings.Contains(b, kw) {
			score += 15 // High score for exact matches
		}
	}

	// Complementary interest matching
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
					score += 10 // Good complementary match
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

	// Semantic similarity (general activity types)
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
			score += 5 // Bonus for same category
		}
	}

	return score
}

// Enhanced location scoring with gradual distance penalties
func calculateLocationScore(userLat, userLon, candidateLat, candidateLon float64, maxRadiusKm int, locationWeight int) int {
	distance := haversine(userLat, userLon, candidateLat, candidateLon)

	// Note: Radius enforcement is now handled at SQL level, so this function
	// focuses purely on proximity-based scoring within the allowed radius

	// Check if outside radius (return 0)
	if maxRadiusKm > 0 && distance > float64(maxRadiusKm) {
		return 0
	}

	// Calculate proximity score (closer = higher score)
	if distance == 0 {
		return locationWeight // Same location = full score
	}

	// If no radius is set (maxRadiusKm = 0), treat all distances equally
	if maxRadiusKm == 0 {
		return locationWeight / 2 // Give moderate score when location matters but no radius set
	}

	// Gradual scoring: closer distances get higher scores
	proximityRatio := 1.0 - (distance / float64(maxRadiusKm))
	proximityScore := int(proximityRatio * float64(locationWeight))

	// Fixed distance-based bonuses for very close matches
	bonus := 0
	if distance <= 5 { // Within 5km
		bonus = 5
	} else if distance <= 15 { // Within 15km
		bonus = 2
	}

	// Return proximity score plus bonus
	// For zero weight, still give bonus points for very close matches
	if locationWeight == 0 && bonus > 0 {
		return bonus
	}

	return proximityScore + bonus
}

// Haversine formula for distance in km
func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371 // Earth radius in km
	dLat := (lat2 - lat1) * (math.Pi / 180)
	dLon := (lon2 - lon1) * (math.Pi / 180)
	lat1 = lat1 * (math.Pi / 180)
	lat2 = lat2 * (math.Pi / 180)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// Enhanced food preference matching
func calculateFoodScore(userFood, candidateFood string) int {
	if userFood == "" || candidateFood == "" {
		return 0
	}

	// Exact match
	if strings.EqualFold(userFood, candidateFood) {
		return 10
	}

	userLower := strings.ToLower(userFood)
	candidateLower := strings.ToLower(candidateFood)

	// Cuisine category matching
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
			return 6 // Partial match within cuisine group
		}
	}

	return 0
}

// Enhanced music preference matching
func calculateMusicScore(userMusic, candidateMusic string) int {
	if userMusic == "" || candidateMusic == "" {
		return 0
	}

	// Exact match
	if strings.EqualFold(userMusic, candidateMusic) {
		return 10
	}

	userLower := strings.ToLower(userMusic)
	candidateLower := strings.ToLower(candidateMusic)

	// Genre category matching
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
			return 6 // Partial match within genre group
		}
	}

	return 0
}

func getRecommendedUserIDs(db *sql.DB, userID int) ([]int, error) {
	results, err := getRecommendationsWithScores(db, userID)
	if err != nil {
		return nil, err
	}

	var recommendations []int
	for _, result := range results {
		recommendations = append(recommendations, result.UserID)
	}
	return recommendations, nil
}

// RecommendationResult represents a user recommendation with calculated score
type RecommendationResult struct {
	UserID          int     `json:"user_id"`
	Score           int     `json:"score"`
	ScorePercentage float64 `json:"score_percentage"`
	Distance        float64 `json:"distance,omitempty"`
}

func getRecommendationsWithScores(db *sql.DB, userID int) ([]RecommendationResult, error) {
	var userProfile Profile
	var analogPassions, digitalDelights, matchPrefsRaw []byte
	err := db.QueryRow(`
        SELECT user_id, analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music,
               location_lat, location_lon, max_radius_km, match_preferences
        FROM profiles WHERE user_id = $1
    `, userID).Scan(
		&userProfile.UserID, &analogPassions, &digitalDelights, &userProfile.CrossPollination,
		&userProfile.FavoriteFood, &userProfile.FavoriteMusic,
		&userProfile.LocationLat, &userProfile.LocationLon, &userProfile.MaxRadiusKm, &matchPrefsRaw,
	)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(analogPassions, &userProfile.AnalogPassions)
	json.Unmarshal(digitalDelights, &userProfile.DigitalDelights)
	json.Unmarshal(matchPrefsRaw, &userProfile.MatchPreferences)

	rows, err := db.Query(`
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

	for rows.Next() {
		var c Profile
		var analog, digital []byte
		err := rows.Scan(&c.UserID, &analog, &digital, &c.CrossPollination, &c.FavoriteFood, &c.FavoriteMusic, &c.LocationLat, &c.LocationLon)
		if err != nil {
			continue
		}
		json.Unmarshal(analog, &c.AnalogPassions)
		json.Unmarshal(digital, &c.DigitalDelights)

		// Calculate distance for radius filtering and scoring
		distance := haversine(userProfile.LocationLat, userProfile.LocationLon, c.LocationLat, c.LocationLon)

		// Enforce radius filtering when radius > 0
		if userProfile.MaxRadiusKm > 0 && distance > float64(userProfile.MaxRadiusKm) {
			continue // Skip candidates outside radius
		}
		json.Unmarshal(analog, &c.AnalogPassions)
		json.Unmarshal(digital, &c.DigitalDelights)

		score := 0

		// Enhanced analog passions scoring
		analogScore := calculateInterestScore(userProfile.AnalogPassions, c.AnalogPassions)
		score += analogScore * userProfile.MatchPreferences["analog_passions"] / 3 // Adjust for new scoring scale

		// Enhanced digital delights scoring
		digitalScore := calculateInterestScore(userProfile.DigitalDelights, c.DigitalDelights)
		score += digitalScore * userProfile.MatchPreferences["digital_delights"] / 3 // Adjust for new scoring scale

		// Enhanced cross-pollination scoring
		crossScore := crossPollinationScore(userProfile.CrossPollination, c.CrossPollination)
		score += crossScore * userProfile.MatchPreferences["collaboration_interests"] / 15 // Adjust for new scoring scale

		// Enhanced food preference scoring
		foodScore := calculateFoodScore(userProfile.FavoriteFood, c.FavoriteFood)
		score += foodScore * userProfile.MatchPreferences["favorite_food"] / 10 // Adjust for new scoring scale

		// Enhanced music preference scoring
		musicScore := calculateMusicScore(userProfile.FavoriteMusic, c.FavoriteMusic)
		score += musicScore * userProfile.MatchPreferences["favorite_music"] / 10 // Adjust for new scoring scale

		// Enhanced location scoring
		if userProfile.MatchPreferences["location"] == 0 {
			// User doesn't care about location at all - no location-based scoring
			// Since we already filtered by radius in SQL (if radius > 0), just don't add location points
		} else {
			// Location preference > 0: Apply proximity-based scoring
			locationScore := calculateLocationScore(
				userProfile.LocationLat, userProfile.LocationLon,
				c.LocationLat, c.LocationLon,
				userProfile.MaxRadiusKm, userProfile.MatchPreferences["location"])
			score += locationScore
		}

		// Calculate percentage for threshold check
		maxPossibleScore := 0
		for _, weight := range userProfile.MatchPreferences {
			maxPossibleScore += weight
		}
		if maxPossibleScore == 0 {
			maxPossibleScore = 1 // Avoid division by zero
		}

		percentage := (float64(score) / float64(maxPossibleScore)) * 100
		if percentage > 100 {
			percentage = 100
		}

		// Only include candidates with at least 25% compatibility
		if percentage >= 25.0 {
			candidates = append(candidates, candidateScore{UserID: c.UserID, Score: score, Distance: distance})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Calculate maximum possible score for percentage calculation (already calculated above)
	maxPossibleScore := 0
	for _, weight := range userProfile.MatchPreferences {
		maxPossibleScore += weight
	}
	if maxPossibleScore == 0 {
		maxPossibleScore = 1 // Avoid division by zero
	}

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
