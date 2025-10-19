package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	_ "github.com/lib/pq"
)

type cfg struct {
	DSN              string
	Count            int
	Seed             int64
	Truncate         bool
	ConnectRate      float64 // proportion of accepted connections
	PendingRate      float64 // proportion of pending connections (me -> them)
	DisconnectedRate float64 // proportion of disconnected connections (previously accepted)
	DismissRate      float64 // proportion of dismissed_recommendations rows per user
	Password         string  // same password for everyone (easy login)
}

func main() {
	var c cfg
	flag.StringVar(&c.DSN, "dsn", os.Getenv("DATABASE_URL"), "Postgres DSN (e.g. postgres://user:pass@localhost:5432/db?sslmode=disable) [env: DATABASE_URL]")
	flag.IntVar(&c.Count, "count", 300, "Number of users to create")
	flag.Int64Var(&c.Seed, "seed", 42, "RNG seed (deterministic)")
	flag.BoolVar(&c.Truncate, "truncate", false, "TRUNCATE target tables before running")
	flag.Float64Var(&c.ConnectRate, "connect-rate", 0.60, "Proportion of accepted connections (0..1)")
	flag.Float64Var(&c.PendingRate, "pending-rate", 0.10, "Proportion of pending connections (0..1)")
	flag.Float64Var(&c.DisconnectedRate, "disconnected-rate", 0.05, "Proportion of disconnected connections (0..1)")
	flag.Float64Var(&c.DismissRate, "dismiss-rate", 0.20, "Proportion of dismissed_recommendations rows per user (0..1)")
	flag.StringVar(&c.Password, "password", "test1234", "Password assigned to all users")
	flag.Parse()

	if c.DSN == "" {
		log.Fatal("Missing DSN: provide --dsn or set DATABASE_URL")
	}
	if c.Count < 1 {
		log.Fatal("--count must be at least 1")
	}
	if c.ConnectRate < 0 || c.ConnectRate > 1 || c.PendingRate < 0 || c.PendingRate > 1 || c.DisconnectedRate < 0 || c.DisconnectedRate > 1 || c.DismissRate < 0 || c.DismissRate > 1 {
		log.Fatal("Rate flags must be in range 0..1")
	}

	r := rand.New(rand.NewSource(c.Seed))

	db, err := sql.Open("postgres", c.DSN)
	if err != nil {
		log.Fatal("DB open error:", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// One big transaction (clear and easy rollback if something breaks constraints)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		log.Fatal("begin tx:", err)
	}
	defer func() {
		// rollback if panic
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if c.Truncate {
		if err := truncateAll(ctx, tx); err != nil {
			_ = tx.Rollback()
			log.Fatal("truncate:", err)
		}
		log.Println("Truncated users, profiles, connections, dismissed_recommendations.")
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(c.Password), bcrypt.DefaultCost)
	if err != nil {
		_ = tx.Rollback()
		log.Fatal("bcrypt:", err)
	}

	// Create users (first two will be our test users)
	userIDs, err := insertUsers(ctx, tx, r, c.Count, string(pwHash))
	if err != nil {
		_ = tx.Rollback()
		log.Fatal("insert users:", err)
	}
	log.Printf("Inserted %d users", len(userIDs))

	if err := insertProfiles(ctx, tx, r, userIDs); err != nil {
		_ = tx.Rollback()
		log.Fatal("insert profiles:", err)
	}
	log.Println("Inserted profiles")

	// Connect first two users if we have at least 2 users
	if len(userIDs) >= 2 {
		if err := connectFirstTwoUsers(ctx, tx, userIDs); err != nil {
			_ = tx.Rollback()
			log.Fatal("connect first two users:", err)
		}
		log.Println("Connected first two users")
	}

	// connections: build a random graph (skip first two users to avoid conflicts)
	if len(userIDs) > 2 {
		if err := insertConnections(ctx, tx, r, userIDs[2:], c.ConnectRate, c.PendingRate, c.DisconnectedRate); err != nil {
			_ = tx.Rollback()
			log.Fatal("insert connections:", err)
		}
		log.Println("Inserted connections (accepted/pending/disconnected)")
	}

	// dismissed_recommendations
	if err := insertDismissedRecommendations(ctx, tx, r, userIDs, c.DismissRate); err != nil {
		_ = tx.Rollback()
		log.Fatal("insert dismissed_recommendations:", err)
	}
	log.Println("Inserted dismissed_recommendations")

	if err := tx.Commit(); err != nil {
		log.Fatal("commit:", err)
	}
	log.Println("Seed complete ✅")
}

func truncateAll(ctx context.Context, tx *sql.Tx) error {
	// NOTE: if FKs enabled, use CASCADE
	_, err := tx.ExecContext(ctx, `
		TRUNCATE TABLE dismissed_recommendations RESTART IDENTITY CASCADE;
		TRUNCATE TABLE connections RESTART IDENTITY CASCADE;
		TRUNCATE TABLE profiles RESTART IDENTITY CASCADE;
		TRUNCATE TABLE users RESTART IDENTITY CASCADE;
	`)
	return err
}

func insertUsers(ctx context.Context, tx *sql.Tx, r *rand.Rand, n int, pwHash string) ([]int, error) {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO users (email, password_hash, last_online) 
		VALUES ($1,$2,$3) 
		ON CONFLICT (email) DO UPDATE SET 
			password_hash = EXCLUDED.password_hash,
			last_online = EXCLUDED.last_online
		RETURNING id`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	emails := make(map[string]struct{}, n)
	ids := make([]int, 0, n)

	// Force first two users to be our test users
	testEmails := []string{"user1@test.local", "user2@test.local"}

	for i := 0; i < n; i++ {
		var email string
		var lastOnline time.Time

		if i < len(testEmails) {
			// Use predefined test emails for first two users
			email = testEmails[i]
			lastOnline = time.Now() // Make test users recently online
		} else {
			// Generate random email for remaining users
			email = uniqueEmail(r, emails)
			lastOnline = time.Now().Add(-time.Duration(r.Intn(14*24)) * time.Hour) // within the last 2 weeks
		}

		var id int
		if err := stmt.QueryRowContext(ctx, email, pwHash, lastOnline).Scan(&id); err != nil {
			return nil, fmt.Errorf("insert user %d (%s): %w", i, email, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func connectFirstTwoUsers(ctx context.Context, tx *sql.Tx, userIDs []int) error {
	// Create single accepted connection from first user to second user
	_, err := tx.ExecContext(ctx, `
		INSERT INTO connections (user_id, target_user_id, status, created_at, updated_at)
		VALUES ($1, $2, 'accepted', NOW(), NOW())
		ON CONFLICT (user_id, target_user_id) DO NOTHING
	`, userIDs[0], userIDs[1])
	if err != nil {
		return fmt.Errorf("connect user %d to %d: %w", userIDs[0], userIDs[1], err)
	}

	return nil
}

func uniqueEmail(r *rand.Rand, used map[string]struct{}) string {
	for {
		local := randomNameSlug(r)
		domain := []string{"example.com", "mail.test", "dev.local"}[r.Intn(3)]
		email := fmt.Sprintf("%s+%d@%s", local, r.Intn(1000000), domain)
		if _, ok := used[email]; !ok {
			used[email] = struct{}{}
			return email
		}
	}
}

func randomNameSlug(r *rand.Rand) string {
	first := []string{"alex", "sam", "mia", "li", "noah", "olivia", "leo", "emil", "sara", "luca", "milla", "mikko", "eeva", "niklas", "sofia"}[r.Intn(15)]
	last := []string{"korhonen", "virtanen", "nieminen", "laine", "heikkinen", "koski", "maki", "aho", "salmi", "rantanen"}[r.Intn(10)]
	return strings.ToLower(fmt.Sprintf("%s.%s", first, last))
}

func insertProfiles(ctx context.Context, tx *sql.Tx, r *rand.Rand, userIDs []int) error {
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO profiles (
			user_id, display_name, about_me, profile_picture_file, location_city, location_lat, location_lon, max_radius_km,
			analog_passions, digital_delights, collaboration_interests, favorite_food, favorite_music, other_bio, match_preferences, is_complete
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15, TRUE
		) ON CONFLICT (user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			about_me = EXCLUDED.about_me,
			profile_picture_file = EXCLUDED.profile_picture_file,
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
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	// Predefined profiles for first two users (test users)
	staticProfiles := []struct {
		displayName            string
		aboutMe                string
		locationCity           string
		locationLat            float64
		locationLon            float64
		maxRadiusKm            int
		analogPassions         []string
		digitalDelights        []string
		collaborationInterests string
		favoriteFood           string
		favoriteMusic          string
		otherBio               map[string]any
	}{
		{
			displayName:            "Test User One",
			aboutMe:                "I'm a passionate creator who loves blending traditional crafts with modern technology. Always looking for interesting collaborations!",
			locationCity:           "Helsinki",
			locationLat:            60.1699,
			locationLon:            24.9384,
			maxRadiusKm:            25,
			analogPassions:         []string{"woodworking", "pottery"},
			digitalDelights:        []string{"3D modeling", "indie games"},
			collaborationInterests: "Looking for maker space enthusiasts and creative collaborators for art-tech projects",
			favoriteFood:           "sushi",
			favoriteMusic:          "electronic",
			otherBio:               map[string]any{"likes_dogs": true, "morning_person": false},
		},
		{
			displayName:            "Test User Two",
			aboutMe:                "Tech enthusiast with a love for analog hobbies. I enjoy connecting digital innovation with hands-on craftsmanship.",
			locationCity:           "Helsinki",
			locationLat:            60.1750,
			locationLon:            24.9400,
			maxRadiusKm:            30,
			analogPassions:         []string{"calligraphy", "blacksmithing"},
			digitalDelights:        []string{"retro computing", "digital art"},
			collaborationInterests: "Seeking creative partnerships in maker communities and art-tech fusion projects",
			favoriteFood:           "pizza",
			favoriteMusic:          "jazz",
			otherBio:               map[string]any{"likes_dogs": true, "morning_person": true},
		},
	}

	cities := []struct {
		City string
		Lat  float64
		Lon  float64
	}{
		{"Helsinki", 60.1699, 24.9384},
		{"Espoo", 60.2055, 24.6559},
		{"Tampere", 61.4978, 23.7610},
		{"Turku", 60.4518, 22.2666},
		{"Oulu", 65.0121, 25.4651},
		{"Jyväskylä", 62.2426, 25.7473},
	}

	foods := []string{"pizza", "sushi", "burgers", "salad", "ramen", "tacos"}
	musics := []string{"rock", "pop", "jazz", "hiphop", "metal", "classical"}

	for i, uid := range userIDs {
		var display, about, food, music, cross string
		var c struct {
			City     string
			Lat, Lon float64
		}
		var lat, lon float64
		var maxRadius int
		var analog, digital []string
		var other map[string]any
		var pic *string

		if i < len(staticProfiles) {
			// Use predefined profile for test users
			profile := staticProfiles[i]
			display = profile.displayName
			about = profile.aboutMe
			c.City = profile.locationCity
			lat = profile.locationLat
			lon = profile.locationLon
			maxRadius = profile.maxRadiusKm
			analog = profile.analogPassions
			digital = profile.digitalDelights
			cross = profile.collaborationInterests
			food = profile.favoriteFood
			music = profile.favoriteMusic
			other = profile.otherBio
			pic = nil // No profile picture for test users initially
		} else {
			// Generate random profile for other users
			c = cities[r.Intn(len(cities))]
			lat = c.Lat + (r.Float64()-0.5)*0.2
			lon = c.Lon + (r.Float64()-0.5)*0.2
			display = displayName(r)
			about = sampleAbout(r)
			maxRadius = 10 + r.Intn(40) // 10..50 km
			analog = []string{pickHobby(r), pickHobby(r)}
			digital = []string{pickDigital(r), pickDigital(r)}
			cross = randomSentence(r)
			food = foods[r.Intn(len(foods))]
			music = musics[r.Intn(len(musics))]
			other = map[string]any{"likes_dogs": r.Intn(2) == 0}

			if r.Float64() < 0.5 {
				s := fmt.Sprintf("profile_%d.jpg", uid)
				pic = &s
			}
		}

		// Match preferences (same for all users for simplicity)
		prefs := mustJSON(map[string]any{
			"analog_passions":         1 + r.Intn(5), // 1-5
			"digital_delights":        1 + r.Intn(5), // 1-5
			"collaboration_interests": 1 + r.Intn(5), // 1-5
			"favorite_food":           1 + r.Intn(3), // 1-3
			"favorite_music":          1 + r.Intn(3), // 1-3
			"location":                3 + r.Intn(3), // 3-5 (higher weight for location)
		})

		analogJSON := mustJSON(analog)
		digitalJSON := mustJSON(digital)
		otherJSON := mustJSON(other)

		if _, err := stmt.ExecContext(ctx,
			uid, display, about, pic, c.City, lat, lon, maxRadius,
			analogJSON, digitalJSON, cross, food, music, otherJSON, prefs,
		); err != nil {
			return fmt.Errorf("insert profile for user %d: %w", uid, err)
		}
	}
	return nil
}

func displayName(r *rand.Rand) string {
	first := []string{"Alex", "Sam", "Mia", "Lauri", "Noah", "Olivia", "Leo", "Emil", "Sara", "Luca", "Milla", "Mikko", "Eeva", "Niklas", "Sofia"}[r.Intn(15)]
	last := []string{"Korhonen", "Virtanen", "Nieminen", "Laine", "Heikkinen", "Koski", "Mäki", "Aho", "Salmi", "Rantanen"}[r.Intn(10)]
	return fmt.Sprintf("%s %s", first, last)
}

func pickHobby(r *rand.Rand) string {
	opts := []string{"hiking", "photography", "cooking", "reading", "board games", "gym", "yoga", "tennis", "golf", "bouldering"}
	return opts[r.Intn(len(opts))]
}
func pickDigital(r *rand.Rand) string {
	opts := []string{"indie games", "retro gaming", "web dev", "3D art", "music production", "photography editing", "AI tinkering", "blogging"}
	return opts[r.Intn(len(opts))]
}

func sampleAbout(r *rand.Rand) string {
	phr := []string{
		"Curious mind, coffee lover.",
		"Weekend hiker and weekday coder.",
		"Always learning new things.",
		"Talk to me about music and tech.",
		"Into analog photography and ramen.",
	}
	return phr[r.Intn(len(phr))]
}

func randomSentence(r *rand.Rand) string {
	parts := []string{"Looking to", "Would love to", "Open to", "Interested to"}
	tails := []string{" build things together.", " jam on ideas.", " meet for a coffee.", " explore new hobbies."}
	return parts[r.Intn(len(parts))] + tails[r.Intn(len(tails))]
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func insertConnections(ctx context.Context, tx *sql.Tx, r *rand.Rand, users []int, connectRate, pendingRate, disconnectedRate float64) error {
	if connectRate == 0 && pendingRate == 0 && disconnectedRate == 0 {
		return nil
	}

	seen := make(map[[2]int]struct{}, len(users)*2)

	// Estimate: create ~count * (connectRate+pendingRate+disconnectedRate) pairs
	targetPairs := int(float64(len(users)) * (connectRate + pendingRate + disconnectedRate) * 1.2)
	if targetPairs < len(users) {
		targetPairs = len(users)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO connections (user_id, target_user_id, status, created_at, updated_at)
		VALUES ($1,$2,$3,NOW(),NOW())
		ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	pickUser := func() (int, int) {
		for {
			a := users[r.Intn(len(users))]
			b := users[r.Intn(len(users))]
			if a == b {
				continue
			}
			key := [2]int{min(a, b), max(a, b)} // “pair” without direction to avoid duplicates for accepted/disconnected
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			return a, b
		}
	}

	insert := func(u, v int, status string) error {
		// for pending, direction matters (u=requester, v=addressee)
		_, err := stmt.ExecContext(ctx, u, v, status)
		return err
	}

	created := 0
	for created < targetPairs {
		u, v := pickUser()
		p := r.Float64()
		switch {
		case p < connectRate:
			// Accepted: direction doesn’t matter; pick random direction
			if r.Intn(2) == 0 {
				if err := insert(u, v, "accepted"); err != nil {
					return err
				}
			} else {
				if err := insert(v, u, "accepted"); err != nil {
					return err
				}
			}
		case p < connectRate+pendingRate:
			// Pending: leave u->v waiting
			if err := insert(u, v, "pending"); err != nil {
				return err
			}
		default:
			// Disconnected: first create accepted, then update to disconnected
			// (for simplicity, no need for 2 inserts in different directions)
			if r.Intn(2) == 0 {
				if err := insert(u, v, "disconnected"); err != nil {
					return err
				}
			} else {
				if err := insert(v, u, "disconnected"); err != nil {
					return err
				}
			}
		}
		created++
	}
	return nil
}

func insertDismissedRecommendations(ctx context.Context, tx *sql.Tx, r *rand.Rand, users []int, rate float64) error {
	if rate <= 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO dismissed_recommendations (user_id, dismissed_user_id)
		VALUES ($1,$2) ON CONFLICT DO NOTHING
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, me := range users {
		// each user gets some dismissed users
		n := int(float64(len(users))*rate*0.2) + r.Intn(3) // light amount per user
		for i := 0; i < n; i++ {
			target := users[r.Intn(len(users))]
			if target == me {
				continue
			}
			if _, err := stmt.ExecContext(ctx, me, target); err != nil {
				return err
			}
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
