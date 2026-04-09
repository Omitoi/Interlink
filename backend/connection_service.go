package main

import (
	"context"
	"database/sql"
	"errors"
)

var (
	ErrInvalidState  = errors.New("invalid_state")
	ErrNotFound      = errors.New("not_found")
	ErrInvalidTarget = errors.New("invalid_target")
)

type ConnectionService interface {
	GetConnections(ctx context.Context, userID int) ([]int, error)
	GetRequests(ctx context.Context, userID int) ([]int, error)

	RequestConnection(ctx context.Context, me, targetID int) (string, *int, error)
	AcceptConnection(ctx context.Context, me, targetID int) (string, *int, error)
	DeclineConnection(ctx context.Context, me, targetID int) (string, error)
	CancelConnection(ctx context.Context, me, targetID int) (string, error)
	DisconnectConnection(ctx context.Context, me, targetID int) (bool, error)
}

type connectionService struct {
	db   *sql.DB
	repo ConnectionRepository
}

func NewConnectionService(db *sql.DB, repo ConnectionRepository) ConnectionService {
	return &connectionService{
		db:   db,
		repo: repo,
	}
}

func (s *connectionService) GetConnections(ctx context.Context, userID int) ([]int, error) {
	return s.repo.GetConnections(ctx, s.db, userID)
}

func (s *connectionService) GetRequests(ctx context.Context, userID int) ([]int, error) {
	return s.repo.GetRequests(ctx, s.db, userID)
}

func (s *connectionService) RequestConnection(ctx context.Context, me, targetID int) (string, *int, error) {
	if me == targetID {
		return "", nil, ErrInvalidTarget
	}

	exists, err := targetExistsAndComplete(ctx, s.db, targetID)
	if err != nil || !exists {
		return "", nil, ErrNotFound
	}

	isRec, err := isCurrentlyRecommendable(ctx, s.db, me, targetID)
	if err != nil {
		return "", nil, err
	}

	var state string
	var connID *int

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		row, err := s.repo.LoadPairForUpdate(tx, me, targetID)
		if err != nil {
			return err
		}

		if row != nil && row.Status == "pending" && row.UserID == targetID && row.TargetUserID == me {
			connID, err = s.repo.UpdateStatus(ctx, tx, row.ID, "accepted")
			if err != nil {
				return err
			}
			state = "accepted"
			return nil
		}

		if row != nil {
			switch row.Status {
			case "pending":
				state = "pending"
				connID = &row.ID
				return nil
			case "accepted":
				state = "accepted"
				connID = &row.ID
				return nil
			case "dismissed", "disconnected":
				return ErrInvalidState
			default:
				return ErrInvalidState
			}
		}

		if !isRec {
			return ErrNotFound
		}

		connID, err = s.repo.CreatePending(ctx, tx, me, targetID)
		if err != nil {
			return err
		}
		state = "pending"
		return nil
	})

	return state, connID, err
}

func (s *connectionService) AcceptConnection(ctx context.Context, me, targetID int) (string, *int, error) {
	if me == targetID {
		return "", nil, ErrInvalidTarget
	}

	exists, err := targetExistsAndComplete(ctx, s.db, targetID)
	if err != nil || !exists {
		return "", nil, ErrNotFound
	}

	var state string
	var connID *int

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		row, err := s.repo.LoadPairForUpdate(tx, me, targetID)
		if err != nil {
			return err
		}
		if row == nil {
			return ErrNotFound
		}

		switch row.Status {
		case "pending":
			if row.UserID == targetID && row.TargetUserID == me {
				connID, err = s.repo.UpdateStatus(ctx, tx, row.ID, "accepted")
				if err != nil {
					return err
				}
				state = "accepted"
				return nil
			}
			return ErrNotFound
		case "accepted":
			state = "accepted"
			connID = &row.ID
			return nil
		case "dismissed", "disconnected":
			return ErrInvalidState
		default:
			return ErrInvalidState
		}
	})

	return state, connID, err
}

func (s *connectionService) DeclineConnection(ctx context.Context, me, targetID int) (string, error) {
	if me == targetID {
		return "", ErrInvalidTarget
	}

	exists, err := targetExistsAndComplete(ctx, s.db, targetID)
	if err != nil || !exists {
		return "", ErrNotFound
	}

	var state string

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		row, err := s.repo.LoadPairForUpdate(tx, me, targetID)
		if err != nil {
			return err
		}
		if row == nil {
			return ErrNotFound
		}

		switch row.Status {
		case "pending":
			if row.UserID == targetID && row.TargetUserID == me {
				_, err = s.repo.UpdateStatus(ctx, tx, row.ID, "dismissed")
				if err != nil {
					return err
				}
				state = "dismissed"
				return nil
			}
			return ErrNotFound
		case "dismissed":
			state = "dismissed"
			return nil
		case "accepted", "disconnected":
			return ErrInvalidState
		default:
			return ErrInvalidState
		}
	})

	return state, err
}

func (s *connectionService) CancelConnection(ctx context.Context, me, targetID int) (string, error) {
	if me == targetID {
		return "", ErrInvalidTarget
	}

	exists, err := targetExistsAndComplete(ctx, s.db, targetID)
	if err != nil || !exists {
		return "", ErrNotFound
	}

	var state string

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		row, err := s.repo.LoadPairForUpdate(tx, me, targetID)
		if err != nil {
			return err
		}
		if row == nil {
			return ErrNotFound
		}

		switch row.Status {
		case "pending":
			if row.UserID == me && row.TargetUserID == targetID {
				_, err = s.repo.UpdateStatus(ctx, tx, row.ID, "dismissed")
				if err != nil {
					return err
				}
				state = "dismissed"
				return nil
			}
			return ErrNotFound
		case "dismissed":
			state = "dismissed"
			return nil
		case "accepted", "disconnected":
			return ErrInvalidState
		default:
			return ErrInvalidState
		}
	})

	return state, err
}

func (s *connectionService) DisconnectConnection(ctx context.Context, me, targetID int) (bool, error) {
	if me == targetID {
		return false, ErrInvalidTarget
	}

	exists, err := targetExistsAndComplete(ctx, s.db, targetID)
	if err != nil || !exists {
		return false, ErrNotFound
	}

	okNoContent := false

	err = withTx(ctx, s.db, func(tx *sql.Tx) error {
		row, err := s.repo.LoadPairForUpdate(tx, me, targetID)
		if err != nil {
			return err
		}
		if row == nil {
			return ErrNotFound
		}

		switch row.Status {
		case "accepted":
			_, err = s.repo.UpdateStatus(ctx, tx, row.ID, "disconnected")
			if err != nil {
				return err
			}
			okNoContent = true
			return nil
		case "disconnected":
			okNoContent = true
			return nil
		case "pending", "dismissed":
			return ErrInvalidState
		default:
			return ErrInvalidState
		}
	})

	return okNoContent, err
}
