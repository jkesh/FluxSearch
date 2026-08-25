package chunker

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"
)

// charsPerToken 与 estimateTokens 一致：中英文混合粗估 4 字符 ≈ 1 token
const charsPerToken = 4

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func estimateTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}
	tokens := n / charsPerToken
	if tokens < 1 {
		return 1
	}
	return tokens
}

func tokenLen(s string) int {
	return estimateTokens(s)
}

func runesForTokens(tokens int) int {
	if tokens <= 0 {
		return 0
	}
	return tokens * charsPerToken
}

func runeLen(s string) int {
	return utf8.RuneCountInString(s)
}

func runeSlice(s string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end <= start {
		return ""
	}
	runes := []rune(s)
	if start >= len(runes) {
		return ""
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
}

func runeTail(s string, n int) string {
	runes := []rune(s)
	if n >= len(runes) {
		return s
	}
	if n <= 0 {
		return ""
	}
	return string(runes[len(runes)-n:])
}

func tokenTail(s string, overlapTokens int) string {
	return runeTail(s, runesForTokens(overlapTokens))
}

func hardSplitByTokens(text string, maxTokens int) []string {
	maxRunes := runesForTokens(maxTokens)
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}
	var parts []string
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}
