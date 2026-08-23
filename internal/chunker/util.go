package chunker

import (
	"crypto/sha256"
	"encoding/hex"
	"unicode/utf8"
)

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func estimateTokens(text string) int {
	n := utf8.RuneCountInString(text)
	if n == 0 {
		return 0
	}
	tokens := n / 4
	if tokens < 1 {
		return 1
	}
	return tokens
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
