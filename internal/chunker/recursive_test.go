package chunker

import (
	"strings"
	"testing"
)

func TestRecursiveChunk_empty(t *testing.T) {
	c := NewRecursive()
	if got := c.Chunk("", DefaultOptions()); len(got) != 0 {
		t.Fatalf("expected 0 chunks, got %d", len(got))
	}
}

func TestRecursiveChunk_shortText(t *testing.T) {
	c := NewRecursive()
	got := c.Chunk("hello world", Options{MaxTokens: 100, OverlapTokens: 0, Separators: DefaultSeparators})
	if len(got) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(got))
	}
	if got[0].Content != "hello world" {
		t.Fatalf("unexpected content: %q", got[0].Content)
	}
	if got[0].ChunkHash == "" {
		t.Fatal("expected chunk hash")
	}
}

func TestRecursiveChunk_paragraphs(t *testing.T) {
	c := NewRecursive()
	text := strings.Repeat("段落内容。", 50) // 250 chars ≈ 62 tokens
	got := c.Chunk(text, Options{MaxTokens: 25, OverlapTokens: 3, Separators: DefaultSeparators})
	if len(got) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(got))
	}
	for i, ch := range got {
		if tokenLen(ch.Content) > 25 {
			t.Fatalf("chunk %d exceeds max tokens: %d", i, tokenLen(ch.Content))
		}
	}
}

func TestRecursiveChunk_overlap(t *testing.T) {
	c := NewRecursive()
	p1 := strings.Repeat("A", 80)
	p2 := strings.Repeat("B", 80)
	text := p1 + "\n\n" + p2
	got := c.Chunk(text, Options{MaxTokens: 25, OverlapTokens: 5, Separators: DefaultSeparators})
	if len(got) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(got))
	}
	if !strings.Contains(got[1].Content, "A") {
		t.Fatalf("expected overlap from previous chunk in second chunk")
	}
}

func TestRecursiveChunk_hardSplit(t *testing.T) {
	c := NewRecursive()
	text := strings.Repeat("x", 250)
	got := c.Chunk(text, Options{MaxTokens: 25, OverlapTokens: 0, Separators: DefaultSeparators})
	if len(got) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(got))
	}
}
