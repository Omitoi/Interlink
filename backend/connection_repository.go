package main

import (
	"context"
	"database/sql"
)

// ConnectionRepository defines the data access methods for connections
type ConnectionRepository interface {
	LoadPairForUpdate(tx *sql.Tx, a, b int) (*ConnectionRow, error)
	CreatePending(ctx context.Context, tx *sql.Tx, requesterID, targetID int) (*int, error)
	UpdateStatus(ctx context.Context, tx *sql.Tx, connectionID int, status string) (*int, error)
	GetConnections(ctx context.Context, db *sql.DB, userID int) ([]int, error)
	GetRequests(ctx context.Context, db *sql.DB, userID int) ([]int, error)
}

type sqlConnectionRepo struct{}

func NewConnectionRepository() ConnectionRepository {
	return &sqlConnectionRepo{}
}

func (r *sqlConnectionRepo) LoadPairForUpdate(tx *sql.Tx, a, b int) (*ConnectionRow, error) {
	// Reusing the existing loadPairForUpdate function from http_common.go
	return loadPairForUpdate(tx, a, b)
}

func (r *sqlConnectionRepo) CreatePending(ctx context.Context, tx *sql.Tx, requesterID, targetID int) (*int, error) {
	var connID int
	err := tx.QueryRowContext(ctx, `
		INSERT INTO connections (user_id, target_user_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING id
	`, requesterID, targetID).Scan(&connID)
	if err != nil {
		return nil, err
	}
	return &connID, nil
}

func (r *sqlConnectionRepo) UpdateStatus(ctx context.Context, tx *sql.Tx, connectionID int, status string) (*int, error) {
	var retID int
	err := tx.QueryRowContext(ctx, `
		UPDATE connections SET status = $1 
		WHERE id = $2 RETURNING id
	`, status, connectionID).Scan(&retID)
	if err != nil {
		return nil, err
	}
	return &retID, nil
}

func (r *sqlConnectionRepo) GetConnections(ctx context.Context, db *sql.DB, userID int) ([]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT 
			CASE 
				WHEN user_id = $1 THEN target_user_id
				ELSE user_id
			END AS connection_id
		FROM connections
		WHERE (user_id = $1 OR target_user_id = $1) AND status = 'accepted'
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connections []int
	for rows.Next() {
		var connID int
		if err := rows.Scan(&connID); err == nil {
			connections = append(connections, connID)
		}
	}
	return connections, nil
}

func (r *sqlConnectionRepo) GetRequests(ctx context.Context, db *sql.DB, userID int) ([]int, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT user_id AS peer_user_id
		FROM connections
		WHERE target_user_id = $1 AND status = 'pending'
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []int
	for rows.Next() {
		var peerID int
		if err := rows.Scan(&peerID); err == nil {
			requests = append(requests, peerID)
		}
	}
	return requests, nil
}
