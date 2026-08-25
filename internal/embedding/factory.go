package embedding

import (
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/config"
)

const (
	ProviderBailian  = "bailian"
	ProviderOllama   = "ollama"
	ProviderLlamaCPP = "llamacpp"
)

type Options struct {
	Provider     string
	APIURL       string
	APIKey       string
	Model        string
	Dimension    int
	BatchSize    int
	MaxLength    int
	LocalBackend string // ollama | llamacpp（provider=local 时）
}

func NewFromConfig(cfg config.Config) (Embedder, error) {
	opts := Options{
		Provider:     cfg.EmbeddingProvider,
		APIURL:       cfg.EmbeddingAPIURL,
		APIKey:       cfg.EmbeddingAPIKey,
		Model:        cfg.EmbeddingModel,
		Dimension:    cfg.EmbeddingDim,
		BatchSize:    cfg.EmbeddingBatchSize,
		MaxLength:    cfg.EmbeddingMaxLength,
		LocalBackend: cfg.EmbeddingLocalBackend,
	}
	return New(opts)
}

func New(opts Options) (Embedder, error) {
	if opts.Provider == "" || opts.Provider == "none" {
		return nil, nil
	}

	switch opts.Provider {
	case ProviderBailian, "dashscope":
		return newBailian(opts)
	case "local":
		switch opts.LocalBackend {
		case "", ProviderOllama:
			return newOllama(opts)
		case ProviderLlamaCPP, "llama.cpp", "llama-cpp":
			return newLlamaCPP(opts)
		default:
			return nil, fmt.Errorf("unknown local embedding backend: %s", opts.LocalBackend)
		}
	case ProviderOllama:
		return newOllama(opts)
	case ProviderLlamaCPP:
		return newLlamaCPP(opts)
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", opts.Provider)
	}
}

func newBailian(opts Options) (Embedder, error) {
	baseURL := opts.APIURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	model := opts.Model
	if model == "" {
		model = "text-embedding-v3"
	}
	if opts.APIKey == "" {
		return nil, fmt.Errorf("bailian embedding requires FLUXSEARCH_EMBEDDING_API_KEY")
	}
	return NewOpenAICompatible(OpenAICompatibleConfig{
		Provider:  ProviderBailian,
		BaseURL:   baseURL,
		APIKey:    opts.APIKey,
		Model:     model,
		Dimension: opts.Dimension,
		BatchSize: opts.BatchSize,
		MaxLength: opts.MaxLength,
	})
}

func newOllama(opts Options) (Embedder, error) {
	baseURL := opts.APIURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	model := opts.Model
	if model == "" {
		model = "bge-m3"
	}
	return NewOllama(OllamaConfig{
		BaseURL:   baseURL,
		Model:     model,
		Dimension: opts.Dimension,
		BatchSize: opts.BatchSize,
	})
}

func newLlamaCPP(opts Options) (Embedder, error) {
	baseURL := opts.APIURL
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8081/v1"
	}
	model := opts.Model
	if model == "" {
		model = "bge-m3"
	}
	return NewOpenAICompatible(OpenAICompatibleConfig{
		Provider:        ProviderLlamaCPP,
		BaseURL:         baseURL,
		APIKey:          opts.APIKey, // llama-server 通常不需要
		Model:           model,
		Dimension:       opts.Dimension,
		BatchSize:       opts.BatchSize,
		MaxLength:       opts.MaxLength,
		HybridSupported: true,
	})
}
