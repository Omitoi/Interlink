package main

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var db *sql.DB

func initDB() {
	// Get database URL from environment variable, fallback to default for development
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		connStr = "user=admin password=password dbname=interlinkdb sslmode=disable"
		log.Default().Println("Warning: DATABASE_URL not set, using default connection string")
	}

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal("Cannot reach the database:", err)
	}
	log.Default().Println("Database connection established successfully")

	// PAUNO's comment 8.8.25:
	// For now, the database structure needs to be created manually using postgre on command line.
	// Later on, maybe here a fallback function, which initializes the db in case it doesn't exist.
}
