package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ChatRepository abstracts all raw SQL for the chat domain.
type ChatRepository interface {
	SaveChatMsg(ctx context.Context, fromUserID, toUserID int, content string) (msgID int64, chatID int, ts time.Time, err error)
	GetChatMessages(ctx context.Context, userID, otherUserID, limit int, before *time.Time) ([]ChatMessage, error)
	MarkChatAsRead(chatID, readerUserID, senderUserID int) error
	GetChatSummaries(ctx context.Context, userID int) ([]ChatPeerSummary, error)
	GetChatIDForPair(ctx context.Context, userID, peerID int) (int, error)
}

type sqlChatRepo struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) ChatRepository {
	return &sqlChatRepo{db: db}
}

func (r *sqlChatRepo) SaveChatMsg(ctx context.Context, fromUserID, toUserID int, content string) (int64, int, time.Time, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		} else {
			_ = tx.Commit()
		}
	}()

	// 1) Verify an accepted connection exists
	var ok int
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM connections
		WHERE status = 'accepted'
			AND ((user_id = $1 AND target_user_id = $2) OR (user_id = $2 AND target_user_id = $1))
		LIMIT 1
	`, fromUserID, toUserID).Scan(&ok)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, time.Time{}, fmt.Errorf("no accepted connection")
		}
		return 0, 0, time.Time{}, err
	}

	// 2) Fetch or create a chat row
	var chatID int
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM chats
		WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
		LIMIT 1
	`, fromUserID, toUserID).Scan(&chatID)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO chats (user1_id, user2_id)
			VALUES (LEAST($1::int, $2::int), GREATEST($1::int, $2::int))
			ON CONFLICT (user1_id, user2_id) DO NOTHING
			RETURNING id
		`, fromUserID, toUserID).Scan(&chatID)
		if err == sql.ErrNoRows {
			// Race condition: refetch
			err = tx.QueryRowContext(ctx, `
				SELECT id
				FROM chats
				WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
				LIMIT 1
			`, fromUserID, toUserID).Scan(&chatID)
		}
	}
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	// 3) Insert message
	var msgID int64
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, `
		INSERT INTO messages (chat_id, sender_id, content)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, chatID, fromUserID, content).Scan(&msgID, &createdAt)
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	// 4) Update unread flags
	_, err = tx.ExecContext(ctx, `
		UPDATE chats c
		SET last_message_at = $3,
			unread_for_user1 = CASE WHEN $2 = c.user2_id THEN TRUE ELSE unread_for_user1 END,
			unread_for_user2 = CASE WHEN $2 = c.user1_id THEN TRUE ELSE unread_for_user2 END
		WHERE c.id = $1
	`, chatID, fromUserID, createdAt)
	if err != nil {
		return 0, 0, time.Time{}, err
	}

	return msgID, chatID, createdAt, nil
}

func (r *sqlChatRepo) GetChatMessages(ctx context.Context, userID, otherUserID, limit int, before *time.Time) ([]ChatMessage, error) {
	// 1) Resolve chat ID
	var chatID int
	err := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM chats
		WHERE user1_id = LEAST($1::int, $2::int) AND user2_id = GREATEST($1::int, $2::int)
		LIMIT 1
	`, userID, otherUserID).Scan(&chatID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []ChatMessage{}, nil
		}
		return nil, err
	}

	// 2) Fetch messages
	q := `
		SELECT id, sender_id, content, created_at
		FROM messages
		WHERE chat_id = $1
			AND ($2::timestamptz IS NULL OR created_at < $2)
			ORDER BY created_at DESC
		LIMIT $3`

	var rows *sql.Rows
	if before != nil {
		rows, err = r.db.QueryContext(ctx, q, chatID, *before, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, q, chatID, nil, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	msgs := make([]ChatMessage, 0, limit)
	for rows.Next() {
		var msgID int64
		var senderID int
		var body string
		var createdAt time.Time
		if err := rows.Scan(&msgID, &senderID, &body, &createdAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, ChatMessage{
			ID:     msgID,
			Type:   "message",
			ChatID: chatID,
			From:   senderID,
			Body:   body,
			Ts:     createdAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return msgs, nil
}

func (r *sqlChatRepo) MarkChatAsRead(chatID, readerUserID, senderUserID int) error {
	_, _ = r.db.Exec(`
		UPDATE messages
		SET is_read = TRUE
		WHERE chat_id = $1 AND sender_id = $2 AND is_read IS FALSE
	`, chatID, senderUserID)

	_, _ = r.db.Exec(`
		UPDATE chats c
		SET unread_for_user1 = CASE WHEN $1 = c.user1_id THEN FALSE ELSE unread_for_user1 END,
			unread_for_user2 = CASE WHEN $1 = c.user2_id THEN FALSE ELSE unread_for_user2 END
		WHERE c.id = $2
	`, readerUserID, chatID)
	return nil
}

func (r *sqlChatRepo) GetChatSummaries(ctx context.Context, userID int) ([]ChatPeerSummary, error) {
	const q = `
WITH accepted AS (
  SELECT CASE WHEN c.user_id = $1 THEN c.target_user_id ELSE c.user_id END AS peer_id
  FROM connections c
  WHERE c.status = 'accepted' AND (c.user_id = $1 OR c.target_user_id = $1)
),
chat_pairs AS (
  SELECT a.peer_id,
         ch.id AS chat_id,
         ch.last_message_at
  FROM accepted a
  LEFT JOIN chats ch
    ON ch.user1_id = LEAST($1::int, a.peer_id)
   AND ch.user2_id = GREATEST($1::int, a.peer_id)
),
unreads AS (
  SELECT cp.peer_id,
         COALESCE(SUM(CASE WHEN m.is_read = FALSE AND m.sender_id = cp.peer_id THEN 1 ELSE 0 END), 0) AS unread_count
  FROM chat_pairs cp
  LEFT JOIN messages m ON m.chat_id = cp.chat_id
  GROUP BY cp.peer_id
)
SELECT
  u.id AS user_id,
  COALESCE(p.display_name, CONCAT('User ', u.id::text)) AS display_name,
  p.profile_picture_file,
  cp.last_message_at,
  COALESCE(uR.unread_count, 0) AS unread_count
FROM accepted a
JOIN users u            ON u.id = a.peer_id
LEFT JOIN profiles p    ON p.user_id = u.id
LEFT JOIN chat_pairs cp ON cp.peer_id = a.peer_id
LEFT JOIN unreads uR    ON uR.peer_id = a.peer_id
ORDER BY COALESCE(cp.last_message_at, to_timestamp(0)) DESC, u.id ASC
;`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summaries := make([]ChatPeerSummary, 0, 32)
	for rows.Next() {
		var s ChatPeerSummary
		var name string
		var pic *string
		var last *time.Time
		var unread int
		if err := rows.Scan(&s.UserID, &name, &pic, &last, &unread); err != nil {
			return nil, err
		}
		s.UserName = name
		s.ProfilePicture = pic
		s.LastMessageAt = last
		s.UnreadMessages = unread
		summaries = append(summaries, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summaries, nil
}

func (r *sqlChatRepo) GetChatIDForPair(ctx context.Context, userID, peerID int) (int, error) {
	var chatID int
	err := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM chats
		WHERE user1_id = LEAST($1::int, $2::int)
		  AND user2_id = GREATEST($1::int, $2::int)
		LIMIT 1
	`, userID, peerID).Scan(&chatID)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return chatID, err
}
