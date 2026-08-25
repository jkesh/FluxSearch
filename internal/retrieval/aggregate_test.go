package retrieval

import (
	"testing"

	"github.com/fluxsearch/fluxsearch/internal/document"
	"github.com/google/uuid"
)

func TestAggregateHitsByDocument(t *testing.T) {
	docA := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	docB := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	chunkA1 := uuid.MustParse("00000000-0000-0000-0000-000000000011")
	chunkA2 := uuid.MustParse("00000000-0000-0000-0000-000000000012")
	chunkB1 := uuid.MustParse("00000000-0000-0000-0000-000000000021")

	hits := []document.SearchHit{
		{ChunkID: chunkA1, DocumentID: docA, Score: 0.5, Content: "a1"},
		{ChunkID: chunkA2, DocumentID: docA, Score: 0.9, Content: "a2"},
		{ChunkID: chunkB1, DocumentID: docB, Score: 0.7, Content: "b1"},
	}

	out := aggregateHitsByDocument(hits)
	if len(out) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(out))
	}
	if out[0].DocumentID != docA || out[0].ChunkID != chunkA2 {
		t.Fatalf("expected docA best chunk a2, got doc=%s chunk=%s", out[0].DocumentID, out[0].ChunkID)
	}
	if out[1].DocumentID != docB {
		t.Fatalf("expected docB second, got %s", out[1].DocumentID)
	}
}
