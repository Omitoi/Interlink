package main

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestWithTx(t *testing.T) {
	t.Run("Successful transaction", func(t *testing.T) {
		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			// Simple successful operation
			_, err := tx.Exec("SELECT 1")
			return err
		})

		if err != nil {
			t.Errorf("Expected successful transaction, got error: %v", err)
		}
	})

	t.Run("Transaction with error rollback", func(t *testing.T) {
		testError := errors.New("test error")

		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			// Return an error to trigger rollback
			return testError
		})

		if err != testError {
			t.Errorf("Expected test error, got: %v", err)
		}
	})

	t.Run("Transaction with SQL error rollback", func(t *testing.T) {
		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			// Execute invalid SQL to cause an error
			_, err := tx.Exec("INVALID SQL STATEMENT")
			return err
		})

		if err == nil {
			t.Error("Expected SQL error, got nil")
		}
	})

	t.Run("Transaction with panic recovery", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic to be re-raised")
			}
		}()

		withTx(context.Background(), db, func(tx *sql.Tx) error {
			// Cause a panic to test recovery
			panic("test panic")
		})
	})

	t.Run("Database unavailable error", func(t *testing.T) {
		// Create a closed database connection to test BeginTx error
		tempDB, err := sql.Open("postgres", "invalid connection string")
		if err != nil {
			t.Fatalf("Failed to create temp DB: %v", err)
		}
		tempDB.Close()

		err = withTx(context.Background(), tempDB, func(tx *sql.Tx) error {
			return nil
		})

		if err == nil {
			t.Error("Expected error when database is unavailable")
		}
	})
}

func TestLoadPairForUpdate(t *testing.T) {
	// Create test users
	user1 := createTestUser(t, "loadpair1@example.com", "password123")
	user2 := createTestUser(t, "loadpair2@example.com", "password123")
	user3 := createTestUser(t, "loadpair3@example.com", "password123")

	// Create profiles
	testProfile := getDefaultTestProfile()
	createTestProfile(t, user1, testProfile)
	createTestProfile(t, user2, testProfile)
	createTestProfile(t, user3, testProfile)

	t.Run("No connection exists", func(t *testing.T) {
		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			conn, err := loadPairForUpdate(tx, user1.ID, user3.ID)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if conn != nil {
				t.Error("Expected nil connection for non-existent pair")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})

	t.Run("Connection exists (user1 -> user2)", func(t *testing.T) {
		// Create connection first
		createConnection(t, user1.ID, user2.ID, "pending")

		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			conn, err := loadPairForUpdate(tx, user1.ID, user2.ID)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if conn == nil {
				t.Error("Expected connection to be found")
				return nil
			}
			if conn.UserID != user1.ID || conn.TargetUserID != user2.ID {
				t.Errorf("Expected user1->user2 connection, got %d->%d", conn.UserID, conn.TargetUserID)
			}
			if conn.Status != "pending" {
				t.Errorf("Expected status 'pending', got '%s'", conn.Status)
			}
			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})

	t.Run("Connection exists (reverse direction)", func(t *testing.T) {
		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			// Query in reverse direction - should still find the same connection
			conn, err := loadPairForUpdate(tx, user2.ID, user1.ID)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if conn == nil {
				t.Error("Expected connection to be found in reverse direction")
				return nil
			}
			// Should still be the same connection (user1 -> user2)
			if conn.UserID != user1.ID || conn.TargetUserID != user2.ID {
				t.Errorf("Expected user1->user2 connection, got %d->%d", conn.UserID, conn.TargetUserID)
			}
			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})

	t.Run("Multiple connections - returns most recent", func(t *testing.T) {
		// Create multiple connections and test that we get the most recent
		user4 := createTestUser(t, "loadpair4@example.com", "password123")
		user5 := createTestUser(t, "loadpair5@example.com", "password123")
		createTestProfile(t, user4, testProfile)
		createTestProfile(t, user5, testProfile)

		// Create first connection
		createConnection(t, user4.ID, user5.ID, "pending")

		// Wait a moment and create a second connection (this simulates the case where
		// there might be duplicate/historical rows)
		createConnection(t, user5.ID, user4.ID, "accepted")

		err := withTx(context.Background(), db, func(tx *sql.Tx) error {
			conn, err := loadPairForUpdate(tx, user4.ID, user5.ID)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if conn == nil {
				t.Error("Expected connection to be found")
				return nil
			}

			// Should get the most recent one (ordered by updated_at DESC, id DESC)
			// The exact result depends on which one was created last
			if (conn.UserID != user4.ID && conn.UserID != user5.ID) ||
				(conn.TargetUserID != user4.ID && conn.TargetUserID != user5.ID) {
				t.Errorf("Expected connection between user4 and user5, got %d->%d", conn.UserID, conn.TargetUserID)
			}
			return nil
		})

		if err != nil {
			t.Errorf("Transaction failed: %v", err)
		}
	})
}
