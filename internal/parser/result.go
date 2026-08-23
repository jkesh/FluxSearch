package parser

// PageContent 单页文本（PDF 等分页格式）
type PageContent struct {
	Page int
	Text string
}

// Result 解析结果
type Result struct {
	SourceType string
	Text       string
	Pages      []PageContent // 非空时表示按页解析（如 PDF）
}
