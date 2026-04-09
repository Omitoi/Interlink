package main

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ChatService encapsulates all business logic for the chat domain.
type ChatService interface {
	SendMessage(ctx context.Context, fromID, toID int, body string) (ChatMessage, error)
	GetHistory(ctx context.Context, userID, otherID, limit int, before *time.Time) ([]ChatMessage, error)
	GetSummaries(ctx context.Context, userID int) ([]ChatPeerSummary, error)
	MarkRead(ctx context.Context, userID, peerID int) error
}

type chatService struct {
	repo ChatRepository
	db   *sql.DB // needed for isOnlineNow presence check
}

func NewChatService(repo ChatRepository, db *sql.DB) ChatService {
	return &chatService{repo: repo, db: db}
}

func (s *chatService) SendMessage(ctx context.Context, fromID, toID int, body string) (ChatMessage, error) {
	msgID, chatID, ts, err := s.repo.SaveChatMsg(ctx, fromID, toID, body)
	if err != nil {
		return ChatMessage{}, err
	}
	return ChatMessage{
		ID:     msgID,
		Type:   "message",
		ChatID: chatID,
		From:   fromID,
		To:     toID,
		Body:   body,
		Ts:     ts,
	}, nil
}

func (s *chatService) GetHistory(ctx context.Context, userID, otherID, limit int, before *time.Time) ([]ChatMessage, error) {
	msgs, err := s.repo.GetChatMessages(ctx, userID, otherID, limit, before)
	if err != nil {
		return nil, err
	}

	// Mark as read (best-effort — don't propagate errors)
	chatID, err := s.repo.GetChatIDForPair(ctx, userID, otherID)
	if err == nil {
		_ = s.repo.MarkChatAsRead(chatID, userID, otherID)
	}

	return msgs, nil
}

func (s *chatService) GetSummaries(ctx context.Context, userID int) ([]ChatPeerSummary, error) {
	summaries, err := s.repo.GetChatSummaries(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Enrich each peer with live presence data
	for i := range summaries {
		online, _ := isOnlineNow(ctx, s.db, summaries[i].UserID)
		summaries[i].IsOnline = online
	}

	return summaries, nil
}

func (s *chatService) MarkRead(ctx context.Context, userID, peerID int) error {
	chatID, err := s.repo.GetChatIDForPair(ctx, userID, peerID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil // no chat yet — nothing to mark
		}
		return err
	}
	return s.repo.MarkChatAsRead(chatID, userID, peerID)
}
