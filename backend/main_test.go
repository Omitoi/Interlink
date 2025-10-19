package main

import (
	"database/sql"
	"log"
	"testing"

	_ "github.com/lib/pq"
)

// Test helper structures and types
type TestUser struct {
	ID       int
	Email    string
	Password string
	Token    string
}

type TestProfile struct {
	DisplayName        string                 `json:"display_name"`
	AboutMe            string                 `json:"about_me"`
	ProfilePictureFile string                 `json:"profile_picture_file"`
	LocationCity       string                 `json:"location_city"`
	LocationLat        float64                `json:"location_lat"`
	LocationLon        float64                `json:"location_lon"`
	MaxRadiusKm        int                    `json:"max_radius_km"`
	AnalogPassions     []string               `json:"analog_passions"`
	DigitalDelights    []string               `json:"digital_delights"`
	CrossPollination   string                 `json:"collaboration_interests"`
	FavoriteFood       string                 `json:"favorite_food"`
	FavoriteMusic      string                 `json:"favorite_music"`
	OtherBio           map[string]interface{} `json:"other_bio"`
	MatchPreferences   map[string]int         `json:"match_preferences"`
}

func TestMain(m *testing.M) {
	var err error
	db, err = sql.Open("postgres", "host=localhost port=5433 user=matchme_user password=matchme_password dbname=matchme_db sslmode=disable")
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	defer db.Close()

	m.Run()
}
