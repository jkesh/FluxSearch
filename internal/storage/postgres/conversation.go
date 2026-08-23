package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/conversation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const conversationSelectCols = `
	id, collection_id, title, status, metadata, created_at, updated_at`

const messageSelectCols = `
	id, conversation_id, role, content, sources, metadata, created_at`

func (s *Store) CreateConversation(ctx context.Context, collectionID uuid.UUID, title string) (conversation.Conversation, error) {
	meta := []byte("{}")
	q := `
		INSERT INTO conversations (collection_id, title, metadata)
		VALUES ($1, $2, $3::jsonb)
		RETURNING ` + conversationSelectCols
	row := s.pool.QueryRow(ctx, q, collectionID, title, meta)
	conv, err := scanConversation(row)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return conv, nil
}

func (s *Store) GetConversation(ctx context.Context, id uuid.UUID) (conversation.Conversation, error) {
	q := `SELECT ` + conversationSelectCols + ` FROM conversations WHERE id = $1`
	row := s.pool.QueryRow(ctx, q, id)
	conv, err := scanConversation(row)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	return conv, nil
}

func (s *Store) ListConversations(ctx context.Context, collectionID uuid.UUID, limit, offset int) ([]conversation.ListItem, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT
			c.id, c.title, c.status, c.created_at, c.updated_at,
			(SELECT COUNT(*)::int FROM messages m WHERE m.conversation_id = c.id) AS message_count,
			COALESCE(
				(SELECT LEFT(content, 100) FROM messages m2
				 WHERE m2.conversation_id = c.id ORDER BY m2.created_at DESC LIMIT 1),
				''
			) AS last_preview
		FROM conversations c
		WHERE c.collection_id = $1 AND c.status = 'active'
		ORDER BY c.updated_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, q, collectionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var items []conversation.ListItem
	for rows.Next() {
		var item conversation.ListItem
		if err := rows.Scan(
			&item.ID, &item.Title, &item.Status, &item.CreatedAt, &item.UpdatedAt,
			&item.MessageCount, &item.LastPreview,
		); err != nil {
			return nil, fmt.Errorf("scan conversation list: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateConversation(ctx context.Context, id uuid.UUID, title *string, status *string) (conversation.Conversation, error) {
	q := `
		UPDATE conversations
		SET
			title = COALESCE($2, title),
			status = COALESCE($3, status),
			updated_at = NOW()
		WHERE id = $1
		RETURNING ` + conversationSelectCols

	row := s.pool.QueryRow(ctx, q, id, title, status)
	conv, err := scanConversation(row)
	if err != nil {
		return conversation.Conversation{}, fmt.Errorf("update conversation: %w", err)
	}
	return conv, nil
}

func (s *Store) TouchConversation(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}
	return nil
}

func (s *Store) DeleteConversation(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) ListMessages(ctx context.Context, conversationID uuid.UUID, limit, offset int) ([]conversation.Message, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	q := `
		SELECT ` + messageSelectCols + `
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, q, conversationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	var msgs []conversation.Message
	for rows.Next() {
		msg, err := scanMessageRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

func (s *Store) ListRecentMessages(ctx context.Context, conversationID uuid.UUID, limit int) ([]conversation.Message, error) {
	if limit <= 0 {
		limit = conversation.MaxHistoryMessages
	}

	q := `
		SELECT ` + messageSelectCols + `
		FROM messages
		WHERE conversation_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := s.pool.Query(ctx, q, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent messages: %w", err)
	}
	defer rows.Close()

	var msgs []conversation.Message
	for rows.Next() {
		msg, err := scanMessageRows(rows)
		if err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}

func (s *Store) AppendMessage(ctx context.Context, conversationID uuid.UUID, role, content string, sources []conversation.Source) (conversation.Message, error) {
	sourcesJSON, err := json.Marshal(sources)
	if err != nil {
		return conversation.Message{}, fmt.Errorf("marshal sources: %w", err)
	}
	if len(sourcesJSON) == 0 || string(sourcesJSON) == "null" {
		sourcesJSON = []byte("[]")
	}
	meta := []byte("{}")

	q := `
		INSERT INTO messages (conversation_id, role, content, sources, metadata)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb)
		RETURNING ` + messageSelectCols

	row := s.pool.QueryRow(ctx, q, conversationID, role, content, sourcesJSON, meta)
	msg, err := scanMessage(row)
	if err != nil {
		return conversation.Message{}, fmt.Errorf("append message: %w", err)
	}

	if err := s.TouchConversation(ctx, conversationID); err != nil {
		return conversation.Message{}, err
	}
	return msg, nil
}

func scanConversation(row pgx.Row) (conversation.Conversation, error) {
	var c conversation.Conversation
	var metaBytes []byte
	err := row.Scan(&c.ID, &c.CollectionID, &c.Title, &c.Status, &metaBytes, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return conversation.Conversation{}, err
	}
	if err := json.Unmarshal(metaBytes, &c.Metadata); err != nil {
		c.Metadata = map[string]any{}
	}
	return c, nil
}

func scanMessage(row pgx.Row) (conversation.Message, error) {
	var m conversation.Message
	var sourcesBytes, metaBytes []byte
	err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &sourcesBytes, &metaBytes, &m.CreatedAt)
	if err != nil {
		return conversation.Message{}, err
	}
	if len(sourcesBytes) > 0 && string(sourcesBytes) != "[]" {
		_ = json.Unmarshal(sourcesBytes, &m.Sources)
	}
	if err := json.Unmarshal(metaBytes, &m.Metadata); err != nil {
		m.Metadata = map[string]any{}
	}
	return m, nil
}

func scanMessageRows(rows pgx.Rows) (conversation.Message, error) {
	var m conversation.Message
	var sourcesBytes, metaBytes []byte
	err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &sourcesBytes, &metaBytes, &m.CreatedAt)
	if err != nil {
		return conversation.Message{}, err
	}
	if len(sourcesBytes) > 0 && string(sourcesBytes) != "[]" {
		_ = json.Unmarshal(sourcesBytes, &m.Sources)
	}
	if err := json.Unmarshal(metaBytes, &m.Metadata); err != nil {
		m.Metadata = map[string]any{}
	}
	return m, nil
}
