package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type openAIClient struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewOpenAICompatible(baseURL, apiKey, model string) (Client, error) {
	if baseURL == "" {
		return nil, fmt.Errorf("llm base url is required")
	}
	if model == "" {
		return nil, fmt.Errorf("llm model is required")
	}
	return &openAIClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: 0}, // streaming: no total timeout
	}, nil
}

func (c *openAIClient) Model() string { return c.model }

func (c *openAIClient) StreamChat(ctx context.Context, messages []Message, opts StreamOptions, onDelta func(string) error) error {
	body := map[string]any{
		"model":    c.model,
		"messages": messages,
		"stream":   true,
	}
	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("llm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm api %d: %s", resp.StatusCode, string(raw))
	}

	return readSSEStream(ctx, resp.Body, onDelta)
}

func readSSEStream(ctx context.Context, r io.Reader, onDelta func(string) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return fmt.Errorf("llm stream error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		text := chunk.Choices[0].Delta.Content
		if text == "" {
			continue
		}
		if err := onDelta(text); err != nil {
			return err
		}
	}
	return scanner.Err()
}
