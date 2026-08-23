package embedding

import "context"

// Embedder 文本向量化（百炼 / Ollama / llama.cpp OpenAI 兼容）
type Embedder interface {
	Provider() string
	Model() string
	Dimension() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
