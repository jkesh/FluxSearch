package chunker

import (
	"strings"
)

// Recursive 递归字符分割 + Overlap
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

	pieces := c.splitRecursive(text, opts.MaxChars, opts.Separators)
	merged := mergePieces(pieces, opts.MaxChars)
	final := applyOverlap(merged, opts.Overlap, opts.MaxChars)

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

func (c *Recursive) splitRecursive(text string, maxChars int, separators []string) []string {
	if runeLen(text) <= maxChars {
		return []string{text}
	}
	if len(separators) == 0 {
		return hardSplit(text, maxChars)
	}

	sep := separators[0]
	rest := separators[1:]

	if sep == "" {
		return hardSplit(text, maxChars)
	}

	parts := strings.Split(text, sep)
	if len(parts) == 1 {
		return c.splitRecursive(text, maxChars, rest)
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

		if runeLen(part) > maxChars {
			out = append(out, c.splitRecursive(part, maxChars, rest)...)
		} else {
			out = append(out, part)
		}
	}
	return out
}

func mergePieces(pieces []string, maxChars int) []string {
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
		if runeLen(candidate) <= maxChars {
			current.WriteString(piece)
			continue
		}

		flush()
		if runeLen(piece) <= maxChars {
			current.WriteString(piece)
		} else {
			merged = append(merged, hardSplit(piece, maxChars)...)
		}
	}
	flush()
	return merged
}

func applyOverlap(chunks []string, overlap, maxChars int) []string {
	if overlap <= 0 || len(chunks) <= 1 {
		return chunks
	}

	out := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		if i == 0 {
			out = append(out, chunk)
			continue
		}
		prefix := runeTail(out[i-1], overlap)
		combined := prefix + chunk
		if runeLen(combined) > maxChars {
			// 截断前缀以保证不超限
			allowed := maxChars - runeLen(chunk)
			if allowed > 0 {
				prefix = runeTail(out[i-1], allowed)
				combined = prefix + chunk
			} else {
				combined = runeSlice(chunk, 0, maxChars)
			}
		}
		out = append(out, combined)
	}
	return out
}

func hardSplit(text string, maxChars int) []string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return []string{text}
	}
	var parts []string
	for start := 0; start < len(runes); start += maxChars {
		end := start + maxChars
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}
