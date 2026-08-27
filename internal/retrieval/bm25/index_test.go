package bm25

import (
	"testing"

	"github.com/google/uuid"
)

func TestBM25SearchBasic(t *testing.T) {
	idx := NewIndex()
	idx.Rebuild([]Document{
		{ChunkID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), DocumentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a1"), Content: "neural networks for image classification"},
		{ChunkID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), DocumentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a2"), Content: "cooking pasta with tomato sauce"},
		{ChunkID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), DocumentID: uuid.MustParse("00000000-0000-0000-0000-0000000000a3"), Content: "deep neural network training tips"},
	})

	hits := idx.Search("neural network", uuid.Nil, 5)
	if len(hits) < 2 {
		t.Fatalf("expected >=2 hits, got %d", len(hits))
	}
	top := hits[0].ChunkID.String()
	if top != "00000000-0000-0000-0000-000000000003" && top != "00000000-0000-0000-0000-000000000001" {
		t.Fatalf("unexpected top hit: %s", top)
	}
}

func TestBM25UpsertDelete(t *testing.T) {
	idx := NewIndex()
	id := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	docID := uuid.MustParse("00000000-0000-0000-0000-0000000000b1")
	idx.Upsert([]Document{{ChunkID: id, DocumentID: docID, Content: "alpha beta gamma"}})
	if idx.Len() != 1 {
		t.Fatalf("len=%d", idx.Len())
	}
	idx.Upsert([]Document{{ChunkID: id, DocumentID: docID, Content: "alpha delta"}})
	if idx.Len() != 1 {
		t.Fatalf("after upsert len=%d", idx.Len())
	}
	hits := idx.Search("delta", uuid.Nil, 5)
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit for delta, got %d", len(hits))
	}
	idx.DeleteByDocument(docID)
	if idx.Len() != 0 {
		t.Fatalf("after delete len=%d", idx.Len())
	}
}
