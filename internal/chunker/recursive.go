package chunker

import (
	"strings"
)

// Recursive 递归分割 + Overlap（按 token 计数）
type Recursive struct{}

func NewRecursive() *Recursive {
	return &Recursive{}
}

func (c *Recursive) Chunk(text string, opts Options) []Result {
	opts = opts.normalized()
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	pieces := c.splitRecursive(text, opts.MaxTokens, opts.Separators)
	merged := mergePieces(pieces, opts.MaxTokens)
	final := applyOverlap(merged, opts.OverlapTokens, opts.MaxTokens)

	results := make([]Result, 0, len(final))
	for _, content := range final {
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}
		results = append(results, Result{
			Index:      len(results),
			Content:    content,
			ChunkHash:  contentHash(content),
			TokenCount: estimateTokens(content),
		})
	}
	return results
}

func (c *Recursive) splitRecursive(text string, maxTokens int, separators []string) []string {
	if tokenLen(text) <= maxTokens {
		return []string{text}
	}
	if len(separators) == 0 {
		return hardSplitByTokens(text, maxTokens)
	}

	sep := separators[0]
	rest := separators[1:]

	if sep == "" {
		return hardSplitByTokens(text, maxTokens)
	}

	parts := strings.Split(text, sep)
	if len(parts) == 1 {
		return c.splitRecursive(text, maxTokens, rest)
	}

	var out []string
	for i, part := range parts {
		if part == "" {
			continue
		}
		// 保留分隔符到前一段末尾（除第一段外）
		if i > 0 && sep != " " {
			part = sep + part
		} else if i > 0 && sep == " " {
			part = " " + part
		}

		if tokenLen(part) > maxTokens {
			out = append(out, c.splitRecursive(part, maxTokens, rest)...)
		} else {
			out = append(out, part)
		}
	}
	return out
}

func mergePieces(pieces []string, maxTokens int) []string {
	var merged []string
	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}
		merged = append(merged, current.String())
		current.Reset()
	}

	for _, piece := range pieces {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}

		if current.Len() == 0 {
			current.WriteString(piece)
			continue
		}

		candidate := current.String() + piece
		if tokenLen(candidate) <= maxTokens {
			current.WriteString(piece)
			continue
		}

		flush()
		if tokenLen(piece) <= maxTokens {
			current.WriteString(piece)
		} else {
			merged = append(merged, hardSplitByTokens(piece, maxTokens)...)
		}
	}
	flush()
	return merged
}

func applyOverlap(chunks []string, overlapTokens, maxTokens int) []string {
	if overlapTokens <= 0 || len(chunks) <= 1 {
		return chunks
	}

	out := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		if i == 0 {
			out = append(out, chunk)
			continue
		}
		prefix := tokenTail(out[i-1], overlapTokens)
		combined := prefix + chunk
		if tokenLen(combined) > maxTokens {
			allowedTokens := maxTokens - tokenLen(chunk)
			if allowedTokens > 0 {
				prefix = tokenTail(out[i-1], allowedTokens)
				combined = prefix + chunk
			} else {
				maxRunes := runesForTokens(maxTokens)
				combined = runeSlice(chunk, 0, maxRunes)
			}
		}
		out = append(out, combined)
	}
	return out
}
