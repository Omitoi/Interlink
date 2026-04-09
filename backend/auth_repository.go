package main

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lib/pq"
)

var (
	ErrEmailExists = errors.New("email_exists")
)

type AuthRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (int, error)
	GetUserByEmail(ctx context.Context, email string) (int, string, error)
	UpdateLastOnline(ctx context.Context, userID int) error
}

type sqlAuthRepo struct {
	db *sql.DB
}

func NewAuthRepository(db *sql.DB) AuthRepository {
	return &sqlAuthRepo{db: db}
}

func (r *sqlAuthRepo) CreateUser(ctx context.Context, email, passwordHash string) (int, error) {
	var newID int
	err := r.db.QueryRowContext(ctx,
		"INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id",
		email, passwordHash,
	).Scan(&newID)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
			return 0, ErrEmailExists
		}
		return 0, err
	}
	return newID, nil
}

func (r *sqlAuthRepo) GetUserByEmail(ctx context.Context, email string) (int, string, error) {
	var userID int
	var passwordHash string
	err := r.db.QueryRowContext(ctx, "SELECT id, password_hash FROM users WHERE email = $1", email).Scan(&userID, &passwordHash)
	if err == sql.ErrNoRows {
		return 0, "", ErrNotFound
	}
	return userID, passwordHash, err
}

func (r *sqlAuthRepo) UpdateLastOnline(ctx context.Context, userID int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET last_online = NOW() WHERE id = $1", userID)
	return err
}
