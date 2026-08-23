package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/conversation"
	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/embedding"
	"github.com/fluxsearch/fluxsearch/internal/llm"
	"github.com/fluxsearch/fluxsearch/internal/settings"
	pgstore "github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"
	"github.com/google/uuid"
)

const defaultMilvusCollection = "fluxsearch_default"

// Source is a retrieved chunk used as RAG context (alias for conversation.Source).
type Source = conversation.Source

type Service struct {
	pg       *pgstore.Store
	milvus   *milvusstore.Store
	embedder embedding.Embedder
	llm      llm.Client
	settings *settings.Manager
}

func NewService(
	pg *pgstore.Store,
	milvus *milvusstore.Store,
	embedder embedding.Embedder,
	llmClient llm.Client,
	settings *settings.Manager,
) *Service {
	return &Service{
		pg:       pg,
		milvus:   milvus,
		embedder: embedder,
		llm:      llmClient,
		settings: settings,
	}
}

func (s *Service) Configure(embedder embedding.Embedder, llmClient llm.Client) {
	s.embedder = embedder
	s.llm = llmClient
}

func (s *Service) retrieve(ctx context.Context, query string) ([]Source, error) {
	if s.embedder == nil || s.milvus == nil {
		return nil, fmt.Errorf("embedding or milvus unavailable")
	}

	cfg := s.settings.Get()
	topK := cfg.SearchTopK
	if topK <= 0 {
		topK = 5
	}

	collectionID, _ := uuid.Parse(document.DefaultCollectionID)
	collectionName := defaultMilvusCollection
	if s.pg != nil {
		if coll, err := s.pg.GetCollectionByID(ctx, collectionID); err == nil {
			collectionName = coll.MilvusCollection
		}
	}

	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, nil
	}

	hits, err := s.milvus.Search(ctx, collectionName, vectors[0], topK)
	if err != nil {
		return nil, fmt.Errorf("milvus search: %w", err)
	}

	docIDs := make([]uuid.UUID, 0, len(hits))
	seen := make(map[uuid.UUID]struct{}, len(hits))
	for _, hit := range hits {
		if _, ok := seen[hit.DocumentID]; ok {
			continue
		}
		seen[hit.DocumentID] = struct{}{}
		docIDs = append(docIDs, hit.DocumentID)
	}

	titles := map[uuid.UUID]string{}
	if s.pg != nil && len(docIDs) > 0 {
		docs, err := s.pg.GetDocumentsByIDs(ctx, docIDs)
		if err == nil {
			for id, doc := range docs {
				titles[id] = doc.Title
			}
		}
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
			Title:      titles[hit.DocumentID],
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
