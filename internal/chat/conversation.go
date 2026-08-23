package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fluxsearch/fluxsearch/internal/conversation"
	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/llm"
	"github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	"github.com/google/uuid"
)

// Outbound WebSocket payloads.
type Outbound struct {
	Type             string   `json:"type"`
	Content          string   `json:"content,omitempty"`
	Sources          []Source `json:"sources,omitempty"`
	Done             bool     `json:"done,omitempty"`
	Error            string   `json:"error,omitempty"`
	ConversationID   string   `json:"conversation_id,omitempty"`
	MessageID        string   `json:"message_id,omitempty"`
}

// Chat runs a full RAG turn with optional conversation persistence.
func (s *Service) Chat(ctx context.Context, req conversation.ChatRequest, send func(Outbound) error) error {
	query := strings.TrimSpace(req.Content)
	if query == "" {
		return fmt.Errorf("empty query")
	}
	if s.pg == nil {
		return s.streamOnly(ctx, query, nil, send)
	}

	collectionID := req.CollectionID
	if collectionID == uuid.Nil {
		collectionID, _ = uuid.Parse(document.DefaultCollectionID)
	}

	var conv conversation.Conversation
	var err error
	created := false

	if req.ConversationID != uuid.Nil {
		conv, err = s.pg.GetConversation(ctx, req.ConversationID)
		if err != nil {
			if postgres.IsNotFound(err) {
				return fmt.Errorf("conversation not found")
			}
			return err
		}
	} else {
		conv, err = s.pg.CreateConversation(ctx, collectionID, "")
		if err != nil {
			return err
		}
		created = true
	}

	if created {
		if err := send(Outbound{
			Type:           "conversation",
			ConversationID: conv.ID.String(),
		}); err != nil {
			return err
		}
	}

	history, err := s.pg.ListRecentMessages(ctx, conv.ID, conversation.MaxHistoryMessages)
	if err != nil {
		return err
	}

	if _, err := s.pg.AppendMessage(ctx, conv.ID, conversation.RoleUser, query, nil); err != nil {
		return err
	}

	if conv.Title == "" {
		title := document.PreviewText(query, 40)
		if title != "" {
			if _, err := s.pg.UpdateConversation(ctx, conv.ID, &title, nil); err != nil {
				return err
			}
		}
	}

	var assistantContent strings.Builder
	var lastSources []Source

	streamErr := s.streamOnly(ctx, query, history, func(out Outbound) error {
		if out.Type == "sources" {
			lastSources = out.Sources
		}
		if out.Type == "token" && out.Content != "" {
			assistantContent.WriteString(out.Content)
		}
		return send(out)
	})
	if streamErr != nil {
		return streamErr
	}

	assistantMsg, err := s.pg.AppendMessage(ctx, conv.ID, conversation.RoleAssistant, assistantContent.String(), lastSources)
	if err != nil {
		return err
	}

	return send(Outbound{
		Type:           "done",
		Done:           true,
		ConversationID: conv.ID.String(),
		MessageID:      assistantMsg.ID.String(),
	})
}

func (s *Service) streamOnly(ctx context.Context, query string, history []conversation.Message, send func(Outbound) error) error {
	sources, err := s.retrieve(ctx, query)
	if err != nil {
		return err
	}

	if err := send(Outbound{Type: "sources", Sources: sources}); err != nil {
		return err
	}

	if s.llm == nil {
		if len(sources) == 0 {
			return send(Outbound{Type: "token", Content: "未找到相关文档，且 LLM 未配置。请在设置页配置 LLM 后重试。"})
		}
		var b strings.Builder
		b.WriteString("LLM 未配置，以下为检索到的相关内容：\n\n")
		for i, src := range sources {
			b.WriteString(fmt.Sprintf("[%d] %s\n%s\n\n", i+1, src.Title, truncate(src.Content, 400)))
		}
		return send(Outbound{Type: "token", Content: b.String()})
	}

	cfg := s.settings.Get()
	messages := []llm.Message{{Role: "system", Content: systemPrompt()}}
	messages = append(messages, historyToLLM(history)...)
	messages = append(messages, llm.Message{Role: "user", Content: buildUserPrompt(query, sources)})

	streamCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	return s.llm.StreamChat(streamCtx, messages, llm.StreamOptions{
		Temperature: cfg.LLMTemperature,
		MaxTokens:   cfg.LLMMaxTokens,
	}, func(delta string) error {
		return send(Outbound{Type: "token", Content: delta})
	})
}

func historyToLLM(msgs []conversation.Message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role != conversation.RoleUser && m.Role != conversation.RoleAssistant {
			continue
		}
		out = append(out, llm.Message{Role: m.Role, Content: m.Content})
	}
	return out
}
