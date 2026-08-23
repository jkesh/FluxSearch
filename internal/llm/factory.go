package llm

import (
	"fmt"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/settings"
)

func NewFromSettings(s settings.AppSettings) (Client, error) {
	provider := strings.ToLower(strings.TrimSpace(s.LLMProvider))
	if provider == "" || provider == "none" {
		return nil, nil
	}

	baseURL := s.LLMAPIURL
	model := s.LLMModel
	apiKey := s.LLMAPIKey

	switch provider {
	case "bailian", "dashscope":
		if baseURL == "" {
			baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
		if apiKey == "" {
			return nil, fmt.Errorf("llm api key required for bailian")
		}
	case "local", "ollama":
		if baseURL == "" {
			baseURL = "http://127.0.0.1:11434/v1"
		}
	case "llamacpp", "llama.cpp", "llama-cpp":
		if baseURL == "" {
			baseURL = "http://127.0.0.1:8081/v1"
		}
	default:
		return nil, fmt.Errorf("unknown llm provider: %s", provider)
	}

	if model == "" {
		model = "qwen-plus"
	}
	return NewOpenAICompatible(baseURL, apiKey, model)
}
