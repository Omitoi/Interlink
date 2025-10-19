package main

import (
	"testing"
)

// ============================================================================
// DATABASE FUNCTIONALITY TEST SUITE
// ============================================================================

func TestDatabaseSuite(t *testing.T) {
	t.Run("Database Initialization", func(t *testing.T) {
		testDatabaseInit(t)
	})
}

func testDatabaseInit(t *testing.T) {
	t.Run("initDB Function", func(t *testing.T) {
		// Test that initDB doesn't panic with valid connection
		// We can't easily test the actual connection setup without mocking,
		// but we can test that the function exists and handles basic cases

		// This is a minimal test to increase coverage
		// In a real scenario, you'd want to test with different connection strings

		// Since initDB is mainly used in main(), we test indirectly by
		// verifying our current db connection works
		if db == nil {
			t.Error("Expected db to be initialized")
		}

		// Test a simple query to verify connection is working
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if err != nil {
			t.Errorf("Database connection test failed: %v", err)
		}

		// This doesn't directly test initDB, but ensures our DB setup is working
		// which is what initDB is responsible for
	})

	t.Run("Database Connection Health", func(t *testing.T) {
		// Test that we can ping the database
		err := db.Ping()
		if err != nil {
			t.Errorf("Database ping failed: %v", err)
		}
	})

	t.Run("Database Schema Verification", func(t *testing.T) {
		// Verify that key tables exist (basic schema test)
		tables := []string{"users", "profiles", "connections", "dismissed_recommendations"}

		for _, table := range tables {
			var exists bool
			query := `SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)`

			err := db.QueryRow(query, table).Scan(&exists)
			if err != nil {
				t.Errorf("Failed to check if table %s exists: %v", table, err)
				continue
			}

			if !exists {
				t.Errorf("Expected table %s to exist", table)
			}
		}
	})
}
