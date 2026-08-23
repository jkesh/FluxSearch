package llm

import (
	"context"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type StreamOptions struct {
	Temperature float64
	MaxTokens   int
}

// Client streams chat completions (OpenAI-compatible).
type Client interface {
	Model() string
	StreamChat(ctx context.Context, messages []Message, opts StreamOptions, onDelta func(string) error) error
}
