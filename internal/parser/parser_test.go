package parser

import (
	"strings"
	"testing"
)

func TestDetectSourceType(t *testing.T) {
	st, err := DetectSourceType("readme.md", "")
	if err != nil || st != "markdown" {
		t.Fatalf("got %q err=%v", st, err)
	}

	st, err = DetectSourceType("doc.pdf", "")
	if err != nil || st != "pdf" {
		t.Fatalf("got %q err=%v", st, err)
	}

	_, err = DetectSourceType("file.docx", "")
	if err == nil {
		t.Fatal("expected error for docx")
	}
}

func TestExtractTextHTML(t *testing.T) {
	text, err := ExtractText("html", []byte("<p>Hello <b>world</b></p>"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "world") {
		t.Fatalf("unexpected: %q", text)
	}
}

func TestNormalizeText(t *testing.T) {
	got := normalizeText("  hello   world \n\n  foo  ")
	if got != "hello world\nfoo" {
		t.Fatalf("unexpected: %q", got)
	}
}

func TestParsePDF_invalid(t *testing.T) {
	_, err := Parse("test.pdf", "", []byte("not a pdf"))
	if err == nil {
		t.Fatal("expected error for invalid pdf")
	}
}
