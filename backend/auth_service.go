package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid_credentials")
)

type AuthService interface {
	Register(ctx context.Context, email, password string) (string, int, error)
	Login(ctx context.Context, email, password string) (string, int, error)
	ValidateToken(tokenStr string) (int, error)
	UpdateLastOnline(ctx context.Context, userID int) error
}

type authService struct {
	repo AuthRepository
}

func NewAuthService(repo AuthRepository) AuthService {
	return &authService{repo: repo}
}

func (s *authService) Register(ctx context.Context, email, password string) (string, int, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return "", 0, errors.New("missing_fields")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", 0, err
	}

	newID, err := s.repo.CreateUser(ctx, email, string(hashedPassword))
	if err != nil {
		return "", 0, err
	}

	_ = s.repo.UpdateLastOnline(ctx, newID)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": newID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}

	return tokenString, newID, nil
}

func (s *authService) Login(ctx context.Context, email, password string) (string, int, error) {
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)

	if email == "" || password == "" {
		return "", 0, errors.New("missing_fields")
	}

	userID, passwordHash, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", 0, ErrInvalidCredentials
		}
		return "", 0, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", 0, ErrInvalidCredentials
	}

	_ = s.repo.UpdateLastOnline(ctx, userID)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", 0, err
	}

	return tokenString, userID, nil
}

func (s *authService) ValidateToken(tokenStr string) (int, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return 0, errors.New("invalid_token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("invalid_token_claims")
	}
	userIDFloat, ok := claims["user_id"].(float64)
	if !ok {
		return 0, errors.New("invalid_user_id_in_token")
	}
	return int(userIDFloat), nil
}

func (s *authService) UpdateLastOnline(ctx context.Context, userID int) error {
	return s.repo.UpdateLastOnline(ctx, userID)
}
