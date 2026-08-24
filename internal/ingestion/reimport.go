package ingestion

import (
	"context"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/parser"
	"github.com/google/uuid"
)

// ReimportFile replaces an existing document's content and re-indexes it.
func (s *Service) ReimportFile(ctx context.Context, documentID uuid.UUID, in ImportInput) (ImportResult, error) {
	if s.pg == nil {
		return ImportResult{}, fmt.Errorf("postgres unavailable")
	}
	if len(in.Raw) == 0 {
		return ImportResult{}, fmt.Errorf("empty content")
	}

	existing, err := s.pg.GetDocument(ctx, documentID)
	if err != nil {
		return ImportResult{}, fmt.Errorf("get document: %w", err)
	}

	parsed, err := parser.Parse(in.Filename, in.SourceType, in.Raw)
	if err != nil {
		return ImportResult{}, err
	}

	contentHash := hashContent(parsed.Text)
	pages := toPageSnapshots(parsed.Pages)
	in.CollectionID = existing.CollectionID
	return s.replaceDocument(ctx, existing, in, parsed, contentHash, pages)
}
