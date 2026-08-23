package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

var supportedTypes = map[string]bool{
	"markdown": true,
	"txt":      true,
	"html":     true,
	"pdf":      true,
}

// DetectSourceType 根据文件名或显式类型识别 source_type
func DetectSourceType(filename, explicit string) (string, error) {
	if explicit != "" {
		explicit = strings.ToLower(strings.TrimSpace(explicit))
		if supportedTypes[explicit] {
			return explicit, nil
		}
		return "", fmt.Errorf("unsupported source_type: %s", explicit)
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))
	switch ext {
	case "md", "markdown":
		return "markdown", nil
	case "txt":
		return "txt", nil
	case "html", "htm":
		return "html", nil
	case "pdf":
		return "pdf", nil
	default:
		if filename == "" {
			return "markdown", nil
		}
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

// Parse 解析文档内容为纯文本
func Parse(filename, explicitType string, data []byte) (Result, error) {
	if len(data) == 0 {
		return Result{}, fmt.Errorf("empty content")
	}

	sourceType, err := DetectSourceType(filename, explicitType)
	if err != nil {
		return Result{}, err
	}

	switch sourceType {
	case "pdf":
		result, err := extractPDF(data)
		if err != nil {
			// 按页失败时尝试整文档提取
			text, plainErr := extractPDFPlain(data)
			if plainErr != nil {
				return Result{}, err
			}
			if text == "" {
				return Result{}, fmt.Errorf("no extractable text in pdf")
			}
			return Result{SourceType: "pdf", Text: text}, nil
		}
		return result, nil
	case "markdown", "txt":
		text := strings.TrimSpace(string(data))
		if text == "" {
			return Result{}, fmt.Errorf("no extractable text")
		}
		return Result{SourceType: sourceType, Text: text}, nil
	case "html":
		text := strings.TrimSpace(stripHTML(string(data)))
		if text == "" {
			return Result{}, fmt.Errorf("no extractable text")
		}
		return Result{SourceType: sourceType, Text: text}, nil
	default:
		return Result{}, fmt.Errorf("parser not implemented for %s", sourceType)
	}
}

// ExtractText 从原始字节提取纯文本（兼容旧调用）
func ExtractText(sourceType string, data []byte) (string, error) {
	result, err := Parse("", sourceType, data)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func SupportedSourceTypes() []string {
	return []string{"markdown", "txt", "html", "pdf"}
}
