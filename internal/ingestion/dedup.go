package ingestion

import (
	"context"
	"fmt"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/settings"
	pgstore "github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	"github.com/google/uuid"
)

const (
	OutcomeCreated = "created"
	OutcomeSkipped = "skipped"
	OutcomeUpdated = "updated"
)

type DedupConfig struct {
	DocumentEnabled       bool
	DocumentMode          string
	DocumentByContentHash bool
	DocumentBySourceURI   bool
	ChunkEnabled          bool
	ChunkScope            string
}

func DedupConfigFromSettings(s settings.AppSettings) DedupConfig {
	d := settings.DedupSettings{
		DocumentDedupEnabled:       s.DocumentDedupEnabled,
		DocumentDedupMode:          s.DocumentDedupMode,
		DocumentDedupByContentHash: s.DocumentDedupByContentHash,
		DocumentDedupBySourceURI:   s.DocumentDedupBySourceURI,
		ChunkDedupEnabled:          s.ChunkDedupEnabled,
		ChunkDedupScope:            s.ChunkDedupScope,
	}.Normalized()
	return DedupConfig{
		DocumentEnabled:       d.DocumentDedupEnabled,
		DocumentMode:          d.DocumentDedupMode,
		DocumentByContentHash: d.DocumentDedupByContentHash,
		DocumentBySourceURI:   d.DocumentDedupBySourceURI,
		ChunkEnabled:          d.ChunkDedupEnabled,
		ChunkScope:            d.ChunkDedupScope,
	}
}

type dedupAction int

const (
	dedupNone dedupAction = iota
	dedupSkip
	dedupReplace
)

func (s *Service) resolveDocumentDedup(
	ctx context.Context,
	collectionID uuid.UUID,
	contentHash, sourceURI string,
) (dedupAction, *document.Document, error) {
	cfg := s.dedup
	if !cfg.DocumentEnabled || s.pg == nil {
		return dedupNone, nil, nil
	}

	if cfg.DocumentByContentHash && contentHash != "" {
		doc, err := s.pg.FindDocumentByContentHash(ctx, collectionID, contentHash)
		if err == nil {
			return dedupSkip, &doc, nil
		}
		if !pgstore.IsNotFound(err) {
			return dedupNone, nil, err
		}
	}

	if cfg.DocumentBySourceURI && sourceURI != "" {
		doc, err := s.pg.FindDocumentBySourceURI(ctx, collectionID, sourceURI)
		if err == nil {
			if doc.ContentHash == contentHash {
				return dedupSkip, &doc, nil
			}
			if cfg.DocumentMode == settings.DocumentDedupModeReplace {
				return dedupReplace, &doc, nil
			}
		} else if !pgstore.IsNotFound(err) {
			return dedupNone, nil, err
		}
	}

	return dedupNone, nil, nil
}

func (s *Service) filterChunkInputs(
	ctx context.Context,
	collectionID, documentID uuid.UUID,
	inputs []document.CreateChunkInput,
) ([]document.CreateChunkInput, int, error) {
	if !s.dedup.ChunkEnabled || len(inputs) == 0 || s.pg == nil {
		return inputs, 0, nil
	}

	hashes := make([]string, 0, len(inputs))
	for _, in := range inputs {
		if in.ChunkHash != "" {
			hashes = append(hashes, in.ChunkHash)
		}
	}
	if len(hashes) == 0 {
		return inputs, 0, nil
	}

	var existing map[string]struct{}
	var err error
	switch s.dedup.ChunkScope {
	case settings.ChunkDedupScopeDocument:
		existing, err = s.pg.ExistingChunkHashesForDocument(ctx, documentID, hashes)
	default:
		existing, err = s.pg.ExistingChunkHashesInCollection(ctx, collectionID, hashes)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("chunk dedup lookup: %w", err)
	}
	if len(existing) == 0 {
		return inputs, 0, nil
	}

	filtered := make([]document.CreateChunkInput, 0, len(inputs))
	skipped := 0
	nextIndex := 0
	for _, in := range inputs {
		if _, dup := existing[in.ChunkHash]; dup {
			skipped++
			continue
		}
		in.ChunkIndex = nextIndex
		filtered = append(filtered, in)
		nextIndex++
	}
	return filtered, skipped, nil
}

func skippedResult(doc document.Document) ImportResult {
	return ImportResult{
		Document: doc,
		Outcome:  OutcomeSkipped,
		Message:  "duplicate document skipped",
	}
}

func updatedResult(doc document.Document, chunks []document.Chunk, vectorsStored bool) ImportResult {
	return ImportResult{
		Document:      doc,
		Chunks:        chunks,
		VectorsStored: vectorsStored,
		Outcome:       OutcomeUpdated,
		Message:       "document updated and re-indexed",
	}
}
