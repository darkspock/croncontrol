// Orchestra chat queries.
// source: orchestra_chat.sql

package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ChatMessage struct {
	ID           string             `json:"id"`
	OrchestraID  string             `json:"orchestra_id"`
	SenderType   string             `json:"sender_type"`
	SenderID     *string            `json:"sender_id"`
	MessageType  string             `json:"message_type"`
	Content      string             `json:"content"`
	Data         []byte             `json:"data,omitempty"`
	CreatedAt    pgtype.Timestamptz `json:"created_at"`
}

const createChatMessage = `INSERT INTO orchestra_chat (id, orchestra_id, sender_type, sender_id, message_type, content, data)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, orchestra_id, sender_type, sender_id, message_type, content, data, created_at`

type CreateChatMessageParams struct {
	ID          string  `json:"id"`
	OrchestraID string  `json:"orchestra_id"`
	SenderType  string  `json:"sender_type"`
	SenderID    *string `json:"sender_id"`
	MessageType string  `json:"message_type"`
	Content     string  `json:"content"`
	Data        []byte  `json:"data,omitempty"`
}

func (q *Queries) CreateChatMessage(ctx context.Context, arg CreateChatMessageParams) (ChatMessage, error) {
	row := q.db.QueryRow(ctx, createChatMessage, arg.ID, arg.OrchestraID, arg.SenderType, arg.SenderID, arg.MessageType, arg.Content, arg.Data)
	var m ChatMessage
	err := row.Scan(&m.ID, &m.OrchestraID, &m.SenderType, &m.SenderID, &m.MessageType, &m.Content, &m.Data, &m.CreatedAt)
	return m, err
}

const listChatMessages = `SELECT id, orchestra_id, sender_type, sender_id, message_type, content, data, created_at
FROM orchestra_chat WHERE orchestra_id = $1 AND created_at > $2 ORDER BY created_at LIMIT $3`

func (q *Queries) ListChatMessages(ctx context.Context, orchestraID string, since time.Time, limit int32) ([]ChatMessage, error) {
	rows, err := q.db.Query(ctx, listChatMessages, orchestraID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChatMessage{}
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.OrchestraID, &m.SenderType, &m.SenderID, &m.MessageType, &m.Content, &m.Data, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

const listChatMessagesAll = `SELECT id, orchestra_id, sender_type, sender_id, message_type, content, data, created_at
FROM orchestra_chat WHERE orchestra_id = $1 ORDER BY created_at LIMIT $2 OFFSET $3`

func (q *Queries) ListChatMessagesAll(ctx context.Context, orchestraID string, limit, offset int32) ([]ChatMessage, error) {
	rows, err := q.db.Query(ctx, listChatMessagesAll, orchestraID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ChatMessage{}
	for rows.Next() {
		var m ChatMessage
		if err := rows.Scan(&m.ID, &m.OrchestraID, &m.SenderType, &m.SenderID, &m.MessageType, &m.Content, &m.Data, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}
