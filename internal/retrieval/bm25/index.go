package bm25

import (
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/google/uuid"
)

const (
	k1 = 1.2
	b  = 0.75
)

// Document is one searchable chunk.
type Document struct {
	ChunkID      uuid.UUID
	DocumentID   uuid.UUID
	CollectionID uuid.UUID
	Content      string
	Page         *int
	Section      string
}

// Hit is a BM25 search result.
type Hit struct {
	ChunkID      uuid.UUID
	DocumentID   uuid.UUID
	CollectionID uuid.UUID
	Content      string
	Score        float32
	Page         *int
	Section      string
}

type posting struct {
	doc  int
	freq int
}

type indexedDoc struct {
	Document
	len int
}

// Index is an in-memory BM25 inverted index.
type Index struct {
	mu       sync.RWMutex
	docs     []indexedDoc
	byChunk  map[uuid.UUID]int
	postings map[string][]posting
	avgLen   float64
	n        int
}

func NewIndex() *Index {
	return &Index{
		byChunk:  make(map[uuid.UUID]int),
		postings: make(map[string][]posting),
	}
}

func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.n
}

// Rebuild replaces the entire index.
func (idx *Index) Rebuild(docs []Document) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.docs = nil
	idx.byChunk = make(map[uuid.UUID]int, len(docs))
	idx.postings = make(map[string][]posting)
	idx.avgLen = 0
	idx.n = 0
	for _, d := range docs {
		idx.upsertLocked(d)
	}
	idx.recomputeAvgLocked()
}

// Upsert inserts or replaces documents by ChunkID.
func (idx *Index) Upsert(docs []Document) {
	if len(docs) == 0 {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	for _, d := range docs {
		idx.upsertLocked(d)
	}
	idx.recomputeAvgLocked()
}

// DeleteByDocument removes all chunks belonging to a document.
func (idx *Index) DeleteByDocument(documentID uuid.UUID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	var chunkIDs []uuid.UUID
	for i, d := range idx.docs {
		if d.DocumentID == documentID && d.ChunkID != uuid.Nil {
			chunkIDs = append(chunkIDs, d.ChunkID)
			_ = i
		}
	}
	for _, id := range chunkIDs {
		idx.deleteChunkLocked(id)
	}
	idx.recomputeAvgLocked()
}

// DeleteByChunk removes a single chunk.
func (idx *Index) DeleteByChunk(chunkID uuid.UUID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.deleteChunkLocked(chunkID)
	idx.recomputeAvgLocked()
}

func (idx *Index) Search(query string, collectionID uuid.UUID, topK int) []Hit {
	if topK <= 0 {
		topK = 10
	}
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if idx.n == 0 {
		return nil
	}

	scores := make(map[int]float64)
	N := float64(idx.n)
	avg := idx.avgLen
	if avg <= 0 {
		avg = 1
	}

	termSeen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		if _, ok := termSeen[term]; ok {
			continue
		}
		termSeen[term] = struct{}{}
		plist := idx.postings[term]
		if len(plist) == 0 {
			continue
		}
		df := float64(len(plist))
		idf := math.Log(1 + (N-df+0.5)/(df+0.5))
		for _, p := range plist {
			doc := idx.docs[p.doc]
			if collectionID != uuid.Nil && doc.CollectionID != uuid.Nil && doc.CollectionID != collectionID {
				continue
			}
			tf := float64(p.freq)
			dl := float64(doc.len)
			denom := tf + k1*(1-b+b*dl/avg)
			scores[p.doc] += idf * (tf * (k1 + 1) / denom)
		}
	}

	type scored struct {
		i int
		s float64
	}
	ranked := make([]scored, 0, len(scores))
	for i, s := range scores {
		if s <= 0 {
			continue
		}
		ranked = append(ranked, scored{i: i, s: s})
	}
	sort.Slice(ranked, func(a, b int) bool {
		if ranked[a].s == ranked[b].s {
			return ranked[a].i < ranked[b].i
		}
		return ranked[a].s > ranked[b].s
	})
	if len(ranked) > topK {
		ranked = ranked[:topK]
	}

	out := make([]Hit, 0, len(ranked))
	for _, r := range ranked {
		d := idx.docs[r.i]
		out = append(out, Hit{
			ChunkID:      d.ChunkID,
			DocumentID:   d.DocumentID,
			CollectionID: d.CollectionID,
			Content:      d.Content,
			Score:        float32(r.s),
			Page:         d.Page,
			Section:      d.Section,
		})
	}
	return out
}

func (idx *Index) upsertLocked(d Document) {
	if d.ChunkID == uuid.Nil {
		return
	}
	if old, ok := idx.byChunk[d.ChunkID]; ok {
		idx.removePostingsLocked(old)
		idx.docs[old] = indexedDoc{} // tombstone; reuse slot
		delete(idx.byChunk, d.ChunkID)
		idx.n--
	}

	tokens := tokenize(d.Content)
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	docIdx := len(idx.docs)
	// Reuse tombstone slots when available to avoid unbounded growth.
	for i := range idx.docs {
		if idx.docs[i].ChunkID == uuid.Nil {
			docIdx = i
			break
		}
	}
	entry := indexedDoc{Document: d, len: len(tokens)}
	if docIdx == len(idx.docs) {
		idx.docs = append(idx.docs, entry)
	} else {
		idx.docs[docIdx] = entry
	}
	idx.byChunk[d.ChunkID] = docIdx
	idx.n++
	for term, freq := range tf {
		idx.postings[term] = append(idx.postings[term], posting{doc: docIdx, freq: freq})
	}
}

func (idx *Index) deleteChunkLocked(chunkID uuid.UUID) {
	docIdx, ok := idx.byChunk[chunkID]
	if !ok {
		return
	}
	idx.removePostingsLocked(docIdx)
	idx.docs[docIdx] = indexedDoc{}
	delete(idx.byChunk, chunkID)
	idx.n--
}

func (idx *Index) removePostingsLocked(docIdx int) {
	d := idx.docs[docIdx]
	if d.ChunkID == uuid.Nil {
		return
	}
	terms := uniqueTerms(tokenize(d.Content))
	for _, term := range terms {
		plist := idx.postings[term]
		if len(plist) == 0 {
			continue
		}
		dst := plist[:0]
		for _, p := range plist {
			if p.doc != docIdx {
				dst = append(dst, p)
			}
		}
		if len(dst) == 0 {
			delete(idx.postings, term)
		} else {
			idx.postings[term] = dst
		}
	}
}

func (idx *Index) recomputeAvgLocked() {
	if idx.n == 0 {
		idx.avgLen = 0
		return
	}
	var sum int
	for _, d := range idx.docs {
		if d.ChunkID == uuid.Nil {
			continue
		}
		sum += d.len
	}
	idx.avgLen = float64(sum) / float64(idx.n)
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var (
		tokens []string
		curr   strings.Builder
	)
	flush := func() {
		if curr.Len() == 0 {
			return
		}
		tok := curr.String()
		curr.Reset()
		if len(tok) < 2 {
			// keep CJK unigrams / short tokens that are meaningful
			runes := []rune(tok)
			if len(runes) == 1 && isCJK(runes[0]) {
				tokens = append(tokens, tok)
			}
			return
		}
		tokens = append(tokens, tok)
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			curr.WriteRune(r)
			continue
		}
		if isCJK(r) {
			flush()
			tokens = append(tokens, string(r))
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r)
}

func uniqueTerms(tokens []string) []string {
	seen := make(map[string]struct{}, len(tokens))
	out := make([]string, 0, len(tokens))
	for _, t := range tokens {
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}
