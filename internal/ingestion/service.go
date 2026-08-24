package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/fluxsearch/fluxsearch/internal/chunker"
	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/fluxsearch/fluxsearch/internal/embedding"
	"github.com/fluxsearch/fluxsearch/internal/parser"
	pgstore "github.com/fluxsearch/fluxsearch/internal/storage/postgres"
	milvusstore "github.com/fluxsearch/fluxsearch/internal/storage/milvus"
	"github.com/google/uuid"
)

type Service struct {
	pg       *pgstore.Store
	milvus   *milvusstore.Store
	embedder embedding.Embedder
	chunker  *chunker.Recursive
	opts     chunker.Options
	dedup    DedupConfig
	objects  ObjectStore
}

func NewService(
	pg *pgstore.Store,
	milvus *milvusstore.Store,
	embedder embedding.Embedder,
	opts chunker.Options,
) *Service {
	if opts.MaxChars <= 0 {
		opts = chunker.DefaultOptions()
	}
	return &Service{
		pg:       pg,
		milvus:   milvus,
		embedder: embedder,
		chunker:  chunker.NewRecursive(),
		opts:     opts,
	}
}

func (s *Service) Configure(embedder embedding.Embedder, opts chunker.Options, dedup DedupConfig, objects ObjectStore) {
	if opts.MaxChars <= 0 {
		opts = chunker.DefaultOptions()
	}
	s.embedder = embedder
	s.opts = opts
	s.dedup = dedup
	s.objects = objects
}

func (s *Service) ReembedDocument(ctx context.Context, documentID uuid.UUID) error {
	if s.pg == nil {
		return fmt.Errorf("postgres unavailable")
	}
	if s.embedder == nil {
		return fmt.Errorf("embedding not configured")
	}

	doc, err := s.pg.GetDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}

	chunks, err := s.pg.ListChunksByDocument(ctx, documentID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return fmt.Errorf("document has no active chunks")
	}

	if err := s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusProcessing, ""); err != nil {
		return err
	}

	_, err = s.indexVectors(ctx, doc, chunks, true)
	if err != nil {
		_ = s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusFailed, err.Error())
		return err
	}

	return s.pg.MarkDocumentIndexed(ctx, doc.ID, len(chunks))
}

func (s *Service) RechunkDocument(ctx context.Context, documentID uuid.UUID) error {
	_, err := s.Rechunk(ctx, documentID)
	return err
}

type ImportInput struct {
	CollectionID uuid.UUID
	Title        string
	Filename     string
	SourceType   string
	Raw          []byte
	Metadata     map[string]any
}

type ImportResult struct {
	Document      document.Document
	Chunks        []document.Chunk
	VectorsStored bool
	Outcome       string
	Message       string
	ChunksSkipped int
}

func (s *Service) Import(ctx context.Context, in ImportInput) (ImportResult, error) {
	if s.pg == nil {
		return ImportResult{}, fmt.Errorf("postgres unavailable")
	}
	if len(in.Raw) == 0 {
		return ImportResult{}, fmt.Errorf("empty content")
	}

	parsed, err := parser.Parse(in.Filename, in.SourceType, in.Raw)
	if err != nil {
		return ImportResult{}, err
	}

	contentHash := hashContent(parsed.Text)
	pages := toPageSnapshots(parsed.Pages)
	sourceURI := in.Filename

	action, existing, err := s.resolveDocumentDedup(ctx, in.CollectionID, contentHash, sourceURI)
	if err != nil {
		return ImportResult{}, err
	}
	switch action {
	case dedupSkip:
		chunks, _ := s.pg.ListChunksByDocument(ctx, existing.ID)
		result := skippedResult(*existing)
		result.Chunks = chunks
		return result, nil
	case dedupReplace:
		return s.replaceDocument(ctx, *existing, in, parsed, contentHash, pages)
	}

	title := in.Title
	if title == "" {
		title = in.Filename
	}
	if title == "" {
		title = "untitled"
	}

	doc, err := s.pg.CreateDocument(ctx, document.CreateDocumentInput{
		CollectionID: in.CollectionID,
		Title:        title,
		SourceType:   parsed.SourceType,
		SourceURI:    in.Filename,
		ContentHash:  contentHash,
		Content:      parsed.Text,
		ContentPages: pages,
		Metadata:     in.Metadata,
	})
	if err != nil {
		return ImportResult{}, fmt.Errorf("create document: %w", err)
	}

	chunks, vectorsStored, err := s.chunkDocument(ctx, doc, parsed.SourceType, parsed.Text, pages, false)
	if err != nil {
		if errors.Is(err, ErrAllChunksDuplicate) {
			_ = s.pg.DeleteDocument(ctx, doc.ID)
			return ImportResult{
				Outcome: OutcomeSkipped,
				Message: "all chunks already indexed in collection",
			}, nil
		}
		return ImportResult{}, err
	}

	if err := s.persistOriginal(ctx, doc, in.Filename, in.Raw); err != nil {
		// non-fatal: content is still in postgres
		_ = err
	}

	doc, err = s.pg.GetDocument(ctx, doc.ID)
	if err != nil {
		return ImportResult{}, err
	}

	return ImportResult{
		Document:      doc,
		Chunks:        chunks,
		VectorsStored: vectorsStored,
		Outcome:       OutcomeCreated,
		Message:       "document imported and chunked",
	}, nil
}

func (s *Service) replaceDocument(
	ctx context.Context,
	existing document.Document,
	in ImportInput,
	parsed parser.Result,
	contentHash string,
	pages []document.PageSnapshot,
) (ImportResult, error) {
	title := in.Title
	if title == "" {
		title = in.Filename
	}
	if title == "" {
		title = existing.Title
	}

	if err := s.pg.UpdateDocumentStatus(ctx, existing.ID, document.StatusProcessing, ""); err != nil {
		return ImportResult{}, err
	}

	doc, err := s.pg.UpdateDocumentContent(ctx, existing.ID, document.UpdateDocumentContentInput{
		Title:        title,
		SourceType:   parsed.SourceType,
		ContentHash:  contentHash,
		Content:      parsed.Text,
		ContentPages: pages,
	})
	if err != nil {
		return ImportResult{}, err
	}

	if err := s.pg.MarkChunksStaleByDocument(ctx, doc.ID); err != nil {
		return ImportResult{}, err
	}

	version, err := s.pg.BumpDocumentVersion(ctx, doc.ID)
	if err != nil {
		return ImportResult{}, err
	}
	doc.Version = version

	chunks, vectorsStored, err := s.chunkDocument(ctx, doc, parsed.SourceType, parsed.Text, pages, true)
	if err != nil {
		return ImportResult{}, err
	}

	doc, err = s.pg.GetDocument(ctx, doc.ID)
	if err != nil {
		return ImportResult{}, err
	}

	_ = s.persistOriginal(ctx, doc, in.Filename, in.Raw)

	return updatedResult(doc, chunks, vectorsStored), nil
}

func (s *Service) Rechunk(ctx context.Context, documentID uuid.UUID) (ImportResult, error) {
	if s.pg == nil {
		return ImportResult{}, fmt.Errorf("postgres unavailable")
	}

	doc, err := s.pg.GetDocument(ctx, documentID)
	if err != nil {
		return ImportResult{}, fmt.Errorf("get document: %w", err)
	}
	if strings.TrimSpace(doc.Content) == "" {
		return ImportResult{}, fmt.Errorf("document has no stored content; please re-import")
	}

	if err := s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusProcessing, ""); err != nil {
		return ImportResult{}, err
	}

	if err := s.pg.MarkChunksStaleByDocument(ctx, doc.ID); err != nil {
		return ImportResult{}, err
	}

	version, err := s.pg.BumpDocumentVersion(ctx, doc.ID)
	if err != nil {
		return ImportResult{}, err
	}
	doc.Version = version

	chunks, vectorsStored, err := s.chunkDocument(ctx, doc, doc.SourceType, doc.Content, doc.ContentPages, true)
	if err != nil {
		return ImportResult{}, err
	}

	doc, err = s.pg.GetDocument(ctx, doc.ID)
	if err != nil {
		return ImportResult{}, err
	}

	return ImportResult{Document: doc, Chunks: chunks, VectorsStored: vectorsStored}, nil
}

func (s *Service) chunkDocument(
	ctx context.Context,
	doc document.Document,
	sourceType, text string,
	pages []document.PageSnapshot,
	replaceVectors bool,
) ([]document.Chunk, bool, error) {
	inputs := s.buildChunkInputs(doc, sourceType, text, pages)
	if len(inputs) == 0 {
		_ = s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusFailed, "no chunks produced")
		return nil, false, fmt.Errorf("no chunks produced")
	}

	filtered, skipped, err := s.filterChunkInputs(ctx, doc.CollectionID, doc.ID, inputs)
	if err != nil {
		_ = s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusFailed, err.Error())
		return nil, false, err
	}
	if len(filtered) == 0 {
		if skipped > 0 {
			return nil, false, ErrAllChunksDuplicate
		}
		msg := "no chunks produced"
		_ = s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusFailed, msg)
		return nil, false, fmt.Errorf("%s", msg)
	}

	created, err := s.pg.CreateChunks(ctx, filtered)
	if err != nil {
		_ = s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusFailed, err.Error())
		return nil, false, fmt.Errorf("save chunks: %w", err)
	}

	vectorsStored, err := s.indexVectors(ctx, doc, created, replaceVectors)
	if err != nil {
		_ = s.pg.UpdateDocumentStatus(ctx, doc.ID, document.StatusFailed, err.Error())
		return nil, false, err
	}

	if err := s.pg.MarkDocumentIndexed(ctx, doc.ID, len(created)); err != nil {
		return nil, vectorsStored, err
	}
	_ = skipped
	return created, vectorsStored, nil
}

func (s *Service) indexVectors(
	ctx context.Context,
	doc document.Document,
	chunks []document.Chunk,
	replace bool,
) (bool, error) {
	if s.embedder == nil || s.milvus == nil {
		return false, nil
	}

	coll, err := s.pg.GetCollectionByID(ctx, doc.CollectionID)
	if err != nil {
		return false, fmt.Errorf("get collection: %w", err)
	}

	if s.embedder != nil {
		s.milvus.SetVectorDim(s.embedder.Dimension())
	}
	if err := s.milvus.EnsureCollection(ctx, coll.MilvusCollection); err != nil {
		return false, fmt.Errorf("ensure milvus collection: %w", err)
	}
	if s.embedder != nil {
		wantDim := s.embedder.Dimension()
		gotDim, err := s.milvus.CollectionVectorDim(ctx, coll.MilvusCollection)
		if err != nil {
			return false, fmt.Errorf("describe milvus collection: %w", err)
		}
		if gotDim != wantDim {
			return false, fmt.Errorf(
				"milvus collection %q dim=%d does not match embedding dim=%d; stop API, then run: FLUXSEARCH_MILVUS_COLLECTION=%s go run ./cmd/ensure-milvus -recreate",
				coll.MilvusCollection, gotDim, wantDim, coll.MilvusCollection,
			)
		}
	}

	if replace {
		if err := s.milvus.DeleteByDocument(ctx, coll.MilvusCollection, doc.ID.String()); err != nil {
			return false, fmt.Errorf("delete old vectors: %w", err)
		}
	}

	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Content
	}

	modelVersion := fmt.Sprintf("%s:%s", s.embedder.Provider(), s.embedder.Model())
	records := make([]milvusstore.VectorRecord, len(chunks))
	idx := s.milvus.IndexConfig()

	if idx.HybridEnabled {
		hybrid, ok := embedding.AsHybrid(s.embedder)
		if !ok {
			return false, fmt.Errorf("hybrid search enabled but embedder does not support sparse vectors")
		}
		vectors, err := hybrid.EmbedHybrid(ctx, texts)
		if err != nil {
			return false, fmt.Errorf("embed chunks: %w", err)
		}
		for i, ch := range chunks {
			var page int64
			if ch.Page != nil {
				page = int64(*ch.Page)
			}
			records[i] = milvusstore.VectorRecord{
				ChunkID:               ch.ID.String(),
				DocumentID:            doc.ID.String(),
				DocumentVersion:       int64(doc.Version),
				Content:               ch.Content,
				Vector:                vectors[i].Dense,
				Sparse:                vectors[i].Sparse,
				Page:                  page,
				Section:               ch.Section,
				EmbeddingModelVersion: modelVersion,
			}
		}
	} else {
		vectors, err := s.embedder.Embed(ctx, texts)
		if err != nil {
			return false, fmt.Errorf("embed chunks: %w", err)
		}
		for i, ch := range chunks {
			var page int64
			if ch.Page != nil {
				page = int64(*ch.Page)
			}
			records[i] = milvusstore.VectorRecord{
				ChunkID:               ch.ID.String(),
				DocumentID:            doc.ID.String(),
				DocumentVersion:       int64(doc.Version),
				Content:               ch.Content,
				Vector:                vectors[i],
				Page:                  page,
				Section:               ch.Section,
				EmbeddingModelVersion: modelVersion,
			}
		}
	}

	if err := s.milvus.InsertVectors(ctx, coll.MilvusCollection, records); err != nil {
		return false, fmt.Errorf("insert milvus: %w", err)
	}

	chunkIDs := make([]uuid.UUID, len(chunks))
	for i, ch := range chunks {
		chunkIDs[i] = ch.ID
	}
	if err := s.pg.UpdateChunksEmbeddingVersion(ctx, chunkIDs, modelVersion); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) buildChunkInputs(
	doc document.Document,
	sourceType, text string,
	pages []document.PageSnapshot,
) []document.CreateChunkInput {
	if len(pages) > 0 {
		return s.chunkByPages(doc, sourceType, pages)
	}
	return s.chunkPlain(doc, sourceType, text)
}

func (s *Service) chunkPlain(doc document.Document, sourceType, text string) []document.CreateChunkInput {
	chunkResults := s.chunker.Chunk(text, s.opts)
	inputs := make([]document.CreateChunkInput, 0, len(chunkResults))
	for _, ch := range chunkResults {
		inputs = append(inputs, document.CreateChunkInput{
			DocumentID:      doc.ID,
			DocumentVersion: doc.Version,
			ChunkIndex:      ch.Index,
			ChunkHash:       ch.ChunkHash,
			Content:         ch.Content,
			TokenCount:      ch.TokenCount,
			Metadata:        map[string]any{"source_type": sourceType},
		})
	}
	return inputs
}

func (s *Service) chunkByPages(doc document.Document, sourceType string, pages []document.PageSnapshot) []document.CreateChunkInput {
	var inputs []document.CreateChunkInput
	chunkIndex := 0

	for _, page := range pages {
		text := strings.TrimSpace(page.Text)
		if text == "" {
			continue
		}

		pageNum := page.Page
		for _, ch := range s.chunker.Chunk(text, s.opts) {
			inputs = append(inputs, document.CreateChunkInput{
				DocumentID:      doc.ID,
				DocumentVersion: doc.Version,
				ChunkIndex:      chunkIndex,
				ChunkHash:       ch.ChunkHash,
				Content:         ch.Content,
				TokenCount:      ch.TokenCount,
				Page:            &pageNum,
				Metadata: map[string]any{
					"source_type": sourceType,
					"page":        pageNum,
				},
			})
			chunkIndex++
		}
	}
	return inputs
}

func toPageSnapshots(pages []parser.PageContent) []document.PageSnapshot {
	if len(pages) == 0 {
		return nil
	}
	out := make([]document.PageSnapshot, len(pages))
	for i, p := range pages {
		out[i] = document.PageSnapshot{Page: p.Page, Text: p.Text}
	}
	return out
}

func hashContent(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
