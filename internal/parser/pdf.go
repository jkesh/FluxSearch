package parser

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

func extractPDF(data []byte) (Result, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Result{}, fmt.Errorf("open pdf: %w", err)
	}

	numPages := reader.NumPage()
	if numPages == 0 {
		return Result{}, fmt.Errorf("pdf has no pages")
	}

	fonts := make(map[string]*pdf.Font)
	var pages []PageContent
	var all strings.Builder

	for i := 1; i <= numPages; i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(fonts)
		if err != nil {
			return Result{}, fmt.Errorf("extract page %d: %w", i, err)
		}

		text = normalizeText(text)
		if text == "" {
			continue
		}

		pages = append(pages, PageContent{Page: i, Text: text})
		if all.Len() > 0 {
			all.WriteString("\n\n")
		}
		all.WriteString(text)
	}

	full := strings.TrimSpace(all.String())
	if full == "" {
		return Result{}, fmt.Errorf("no extractable text in pdf")
	}

	return Result{
		SourceType: "pdf",
		Text:       full,
		Pages:      pages,
	}, nil
}

// extractPDFPlain 整文档一次性提取（部分 PDF 按页提取失败时的兜底）
func extractPDFPlain(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	r, err := reader.GetPlainText()
	if err != nil {
		return "", err
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return normalizeText(string(b)), nil
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == '\n' {
			b.WriteRune('\n')
			prevSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}

	lines := strings.Split(strings.TrimSpace(b.String()), "\n")
	var compact []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			compact = append(compact, line)
		}
	}
	return strings.Join(compact, "\n")
}
