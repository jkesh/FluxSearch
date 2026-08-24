package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/conversation"
	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/llm"
	"github.com/fluxsearch/fluxsearch/internal/retrieval"
	"github.com/fluxsearch/fluxsearch/internal/settings"
	pgstore "github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	"github.com/google/uuid"
)

// Source is a retrieved chunk used as RAG context (alias for conversation.Source).
type Source = conversation.Source

type Service struct {
	pg        *pgstore.Store
	retrieval *retrieval.Service
	llm       llm.Client
	settings  *settings.Manager
}

func NewService(
	pg *pgstore.Store,
	retrievalSvc *retrieval.Service,
	llmClient llm.Client,
	settings *settings.Manager,
) *Service {
	return &Service{
		pg:        pg,
		retrieval: retrievalSvc,
		llm:       llmClient,
		settings:  settings,
	}
}

func (s *Service) Configure(retrievalSvc *retrieval.Service, llmClient llm.Client) {
	s.retrieval = retrievalSvc
	s.llm = llmClient
}

func (s *Service) retrieve(ctx context.Context, query string) ([]Source, error) {
	if s.retrieval == nil {
		return nil, fmt.Errorf("retrieval unavailable")
	}

	cfg := s.settings.Get()
	topK := cfg.SearchTopK
	if topK <= 0 {
		topK = 5
	}

	collectionID, _ := uuid.Parse(document.DefaultCollectionID)
	hits, _, err := s.retrieval.Search(ctx, collectionID, query, topK)
	if err != nil {
		return nil, err
	}

	threshold := float32(cfg.SearchScoreThreshold)
	out := make([]Source, 0, len(hits))
	for _, hit := range hits {
		if threshold > 0 && hit.Score < threshold {
			continue
		}
		out = append(out, Source{
			ChunkID:    hit.ChunkID,
			DocumentID: hit.DocumentID,
			Title:      hit.DocumentTitle,
			Content:    hit.Content,
			Score:      hit.Score,
			Page:       hit.Page,
		})
	}
	return out, nil
}

func systemPrompt() string {
	return `你是 FluxSearch 知识库助手。根据用户提供的「参考资料」回答问题。
规则：
1. 仅依据参考资料作答；资料不足时明确说明，不要编造。
2. 回答使用简体中文，条理清晰。
3. 适当引用资料中的关键信息；无需标注引用编号，除非用户要求。`
}

func buildUserPrompt(query string, sources []Source) string {
	var b strings.Builder
	b.WriteString("## 参考资料\n\n")
	if len(sources) == 0 {
		b.WriteString("（未检索到相关文档片段）\n\n")
	} else {
		for i, src := range sources {
			title := src.Title
			if title == "" {
				title = "未命名文档"
			}
			b.WriteString(fmt.Sprintf("### [%d] %s\n", i+1, title))
			if src.Page != nil && *src.Page > 0 {
				b.WriteString(fmt.Sprintf("页码: %d\n", *src.Page))
			}
			b.WriteString(src.Content)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("## 用户问题\n\n")
	b.WriteString(query)
	return b.String()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
