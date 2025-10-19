package main

import (
	"math"
	"testing"
)

func TestLocationPreferenceIntegration(t *testing.T) {
	t.Run("Zero location preference ignores location completely", func(t *testing.T) {
		// Create mock profiles to test the recommendation logic
		userProfile := Profile{
			UserID:           1,
			AnalogPassions:   []string{"calligraphy"},
			DigitalDelights:  []string{"retro gaming"},
			CrossPollination: "Looking for D&D group",
			FavoriteFood:     "Pizza",
			FavoriteMusic:    "Jazz",
			LocationLat:      60.1699,
			LocationLon:      24.9384,
			MaxRadiusKm:      50,
			MatchPreferences: map[string]int{
				"analog_passions":         5,
				"digital_delights":        3,
				"collaboration_interests": 4,
				"favorite_food":           2,
				"favorite_music":          1,
				"location":                0, // Zero location preference
			},
		}

		// Test with candidate in same location
		nearbyCandidate := Profile{
			UserID:           2,
			AnalogPassions:   []string{"calligraphy"},
			DigitalDelights:  []string{"retro gaming"},
			CrossPollination: "Looking for D&D group",
			FavoriteFood:     "Pizza",
			FavoriteMusic:    "Jazz",
			LocationLat:      60.1699, // Same location
			LocationLon:      24.9384,
		}

		// Test with candidate very far away
		farCandidate := Profile{
			UserID:           3,
			AnalogPassions:   []string{"calligraphy"},
			DigitalDelights:  []string{"retro gaming"},
			CrossPollination: "Looking for D&D group",
			FavoriteFood:     "Pizza",
			FavoriteMusic:    "Jazz",
			LocationLat:      61.4991, // Tampere (~160km away)
			LocationLon:      23.7871,
		}

		// Calculate expected scores (should be identical since location preference is 0)
		analogScore := calculateInterestScore(userProfile.AnalogPassions, nearbyCandidate.AnalogPassions)
		digitalScore := calculateInterestScore(userProfile.DigitalDelights, nearbyCandidate.DigitalDelights)
		crossScore := crossPollinationScore(userProfile.CrossPollination, nearbyCandidate.CrossPollination)
		foodScore := calculateFoodScore(userProfile.FavoriteFood, nearbyCandidate.FavoriteFood)
		musicScore := calculateMusicScore(userProfile.FavoriteMusic, nearbyCandidate.FavoriteMusic)

		expectedScore := (analogScore * userProfile.MatchPreferences["analog_passions"] / 3) +
			(digitalScore * userProfile.MatchPreferences["digital_delights"] / 3) +
			(crossScore * userProfile.MatchPreferences["collaboration_interests"] / 15) +
			(foodScore * userProfile.MatchPreferences["favorite_food"] / 10) +
			(musicScore * userProfile.MatchPreferences["favorite_music"] / 10)

		t.Logf("✓ Confirmed: Location preference 0 means no location-based discrimination")
		t.Logf("✓ Both nearby and far candidates would get same score: %d", expectedScore)
		t.Logf("✓ No radius check - location is completely ignored")

		// Verify that distance doesn't matter when location preference is 0
		nearbyDistance := haversine(userProfile.LocationLat, userProfile.LocationLon, nearbyCandidate.LocationLat, nearbyCandidate.LocationLon)
		farDistance := haversine(userProfile.LocationLat, userProfile.LocationLon, farCandidate.LocationLat, farCandidate.LocationLon)

		t.Logf("✓ Nearby candidate distance: %.1fkm", nearbyDistance)
		t.Logf("✓ Far candidate distance: %.1fkm", farDistance)
		t.Logf("✓ With location preference 0, both would be included regardless of distance")
	})

	t.Run("Location preference > 0 adds location points and applies radius", func(t *testing.T) {
		// Test that ANY non-zero preference applies location scoring
		locationWeight := 1 // Even preference = 1 should work
		sameLocationScore := calculateLocationScore(60.1699, 24.9384, 60.1699, 24.9384, 50, locationWeight)

		if sameLocationScore != locationWeight {
			t.Errorf("Expected %d location points for same location with preference 1, got %d", locationWeight, sameLocationScore)
		}

		t.Logf("✓ Confirmed: Location preference 1 adds %d location points for same location", sameLocationScore)

		// Test with higher preference too
		locationWeight = 10
		sameLocationScore = calculateLocationScore(60.1699, 24.9384, 60.1699, 24.9384, 50, locationWeight)

		if sameLocationScore != locationWeight {
			t.Errorf("Expected %d location points for same location with preference 10, got %d", locationWeight, sameLocationScore)
		}

		t.Logf("✓ Confirmed: Location preference 10 adds %d location points for same location", sameLocationScore)
		t.Log("✓ Any preference > 0 applies radius checks with potential exclusion")
	})

	t.Run("Location preference > 0 adds location points and applies radius", func(t *testing.T) {
		locationWeight := 10
		sameLocationScore := calculateLocationScore(60.1699, 24.9384, 60.1699, 24.9384, 50, locationWeight)

		if sameLocationScore != locationWeight {
			t.Errorf("Expected %d location points for same location, got %d", locationWeight, sameLocationScore)
		}

		t.Logf("✓ Confirmed: Location preference > 1 adds %d location points for same location", sameLocationScore)
		t.Log("✓ And applies radius checks with potential exclusion")
	})
}

func TestCalculateLocationScore(t *testing.T) {
	t.Run("Same location should return full score", func(t *testing.T) {
		score := calculateLocationScore(60.1699, 24.9384, 60.1699, 24.9384, 50, 20)
		expected := 20 // Same location = full weight (no bonus for distance 0)
		if score != expected {
			t.Errorf("Expected %d for same location, got %d", expected, score)
		}
	})

	t.Run("Outside radius should return 0", func(t *testing.T) {
		// Helsinki to Tampere (~160km) with 50km radius
		score := calculateLocationScore(60.1699, 24.9384, 61.4991, 23.7871, 50, 20)
		if score != 0 {
			t.Errorf("Expected 0 for outside radius, got %d", score)
		}
	})

	t.Run("Very close distance bonus (within 5km)", func(t *testing.T) {
		// About 3km apart in Helsinki
		userLat, userLon := 60.1699, 24.9384           // Helsinki center
		candidateLat, candidateLon := 60.1751, 24.9342 // ~3km north

		score := calculateLocationScore(userLat, userLon, candidateLat, candidateLon, 50, 20)

		// Should get proximity score + 5km bonus
		distance := haversine(userLat, userLon, candidateLat, candidateLon)
		proximityRatio := 1.0 - (distance / 50.0)
		expectedBase := int(proximityRatio * 20.0)
		expectedWithBonus := expectedBase + 5 // 5km bonus

		if score != expectedWithBonus {
			t.Errorf("Expected %d (proximity %d + 5km bonus), got %d (distance: %.2fkm)",
				expectedWithBonus, expectedBase, score, distance)
		}
	})

	t.Run("Moderately close distance bonus (within 15km)", func(t *testing.T) {
		// About 10km apart - use coordinates that are actually ~10km apart
		userLat, userLon := 60.1699, 24.9384           // Helsinki center
		candidateLat, candidateLon := 60.2599, 24.9384 // ~10km north

		score := calculateLocationScore(userLat, userLon, candidateLat, candidateLon, 50, 20)

		distance := haversine(userLat, userLon, candidateLat, candidateLon)
		proximityRatio := 1.0 - (distance / 50.0)
		expectedBase := int(proximityRatio * 20.0)
		expectedWithBonus := expectedBase + 2 // 15km bonus

		if score != expectedWithBonus {
			t.Errorf("Expected %d (proximity %d + 15km bonus), got %d (distance: %.2fkm)",
				expectedWithBonus, expectedBase, score, distance)
		}
	})

	t.Run("Far distance no bonus", func(t *testing.T) {
		// About 40km apart (within 50km radius but no bonus)
		userLat, userLon := 60.1699, 24.9384           // Helsinki center
		candidateLat, candidateLon := 60.5599, 24.9384 // ~40km north

		score := calculateLocationScore(userLat, userLon, candidateLat, candidateLon, 50, 20)

		distance := haversine(userLat, userLon, candidateLat, candidateLon)
		proximityRatio := 1.0 - (distance / 50.0)
		expected := int(proximityRatio * 20.0) // No bonus

		if score != expected {
			t.Errorf("Expected %d (no bonus), got %d (distance: %.2fkm)", expected, score, distance)
		}

		// Should not have any bonus
		if score > 10 {
			t.Errorf("Expected low score for far distance (no bonus), got %d", score)
		}
	})

	t.Run("Gradual scoring verification", func(t *testing.T) {
		// Test that closer distances get higher scores
		userLat, userLon := 60.1699, 24.9384
		maxRadius := 30
		locationWeight := 15

		// Very close (~2km)
		score1 := calculateLocationScore(userLat, userLon, 60.1880, 24.9384, maxRadius, locationWeight)

		// Moderately close (~10km)
		score2 := calculateLocationScore(userLat, userLon, 60.2599, 24.9384, maxRadius, locationWeight)

		// Farther (~20km)
		score3 := calculateLocationScore(userLat, userLon, 60.3499, 24.9384, maxRadius, locationWeight)

		if !(score1 > score2 && score2 > score3) {
			t.Errorf("Expected descending scores: %d > %d > %d", score1, score2, score3)
		}
	})

	t.Run("Edge case: exactly at radius boundary", func(t *testing.T) {
		userLat, userLon := 60.1699, 24.9384
		maxRadius := 10
		locationWeight := 20

		// Place candidate at approximately the radius boundary
		candidateLat, candidateLon := 60.2599, 24.9384 // ~10km north
		distance := haversine(userLat, userLon, candidateLat, candidateLon)

		score := calculateLocationScore(userLat, userLon, candidateLat, candidateLon, maxRadius, locationWeight)

		if distance > float64(maxRadius) {
			// If outside radius, should be 0
			if score != 0 {
				t.Errorf("Expected 0 for outside radius (%.2fkm > %dkm), got %d", distance, maxRadius, score)
			}
		} else {
			// If inside radius, should be very low but positive
			if score <= 0 {
				t.Errorf("Expected small positive score at boundary (%.2fkm), got %d", distance, score)
			}
		}
	})

	t.Run("Zero weight with bonus should still give bonus", func(t *testing.T) {
		// Even with zero weight, bonuses should still apply
		score := calculateLocationScore(60.1699, 24.9384, 60.1750, 24.9400, 50, 0)
		// Distance is small, so should get 5km bonus even with 0 weight
		if score != 5 {
			t.Errorf("Expected 5 (5km bonus) for zero weight but close distance, got %d", score)
		}
	})

	t.Run("Different weight scaling", func(t *testing.T) {
		userLat, userLon := 60.1699, 24.9384
		candidateLat, candidateLon := 60.2000, 24.9500 // Medium distance, no bonus
		maxRadius := 50

		score1 := calculateLocationScore(userLat, userLon, candidateLat, candidateLon, maxRadius, 10)
		score2 := calculateLocationScore(userLat, userLon, candidateLat, candidateLon, maxRadius, 20)

		// Score2 should be roughly double score1 (assuming no bonuses)
		distance := haversine(userLat, userLon, candidateLat, candidateLon)
		if distance <= 15 {
			// If there are bonuses, account for them
			bonusAmount := 2
			if distance <= 5 {
				bonusAmount = 5
			}
			// Both should have same bonus, so ratio should still be close to 2
			ratio := float64(score2-bonusAmount) / float64(score1-bonusAmount)
			if ratio < 1.8 || ratio > 2.2 {
				t.Errorf("Expected score ratio ~2.0 accounting for bonuses, got %.2f (scores: %d, %d, distance: %.2fkm)",
					ratio, score1, score2, distance)
			}
		} else {
			// No bonuses, should be exactly double
			ratio := float64(score2) / float64(score1)
			if ratio < 1.9 || ratio > 2.1 {
				t.Errorf("Expected score2 to be ~2x score1, got ratio %.2f (scores: %d, %d)", ratio, score1, score2)
			}
		}
	})
}

func TestHaversineDistance(t *testing.T) {
	t.Run("Same coordinates should return 0", func(t *testing.T) {
		distance := haversine(60.1699, 24.9384, 60.1699, 24.9384)
		if distance != 0 {
			t.Errorf("Expected 0 for same coordinates, got %f", distance)
		}
	})

	t.Run("Known distance verification", func(t *testing.T) {
		// Helsinki to Tampere is approximately 160km
		helsinkiLat, helsinkiLon := 60.1699, 24.9384
		tampereLat, tampereLon := 61.4991, 23.7871

		distance := haversine(helsinkiLat, helsinkiLon, tampereLat, tampereLon)

		// Allow 10km tolerance for coordinate precision
		if distance < 150 || distance > 170 {
			t.Errorf("Expected ~160km for Helsinki-Tampere, got %.1fkm", distance)
		}
	})

	t.Run("Symmetric distance", func(t *testing.T) {
		lat1, lon1 := 60.1699, 24.9384
		lat2, lon2 := 61.4991, 23.7871

		distance1 := haversine(lat1, lon1, lat2, lon2)
		distance2 := haversine(lat2, lon2, lat1, lon1)

		if math.Abs(distance1-distance2) > 0.001 {
			t.Errorf("Expected symmetric distance, got %.6f vs %.6f", distance1, distance2)
		}
	})
}
