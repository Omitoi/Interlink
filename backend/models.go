package main

// Profile represents a user's profile with preferences for matching
type Profile struct {
	UserID           int
	AnalogPassions   []string
	DigitalDelights  []string
	CrossPollination string
	FavoriteFood     string
	FavoriteMusic    string
	LocationLat      float64
	LocationLon      float64
	MaxRadiusKm      int
	MatchPreferences map[string]int
}
