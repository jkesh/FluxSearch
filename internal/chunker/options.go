package chunker

// DefaultMaxTokens 默认单块最大 token 数（与 BGE-M3 常用 max_length 对齐）
const DefaultMaxTokens = 512

// DefaultOverlapTokens 默认块间重叠 token 数
const DefaultOverlapTokens = 64

// DefaultSeparators 递归分割优先级：段落 → 行 → 中英文句号 → 空格 → 硬切
var DefaultSeparators = []string{"\n\n", "\n", "。", ".", " ", ""}

type Options struct {
	MaxTokens   int
	OverlapTokens int
	Separators  []string
}

func DefaultOptions() Options {
	return Options{
		MaxTokens:     DefaultMaxTokens,
		OverlapTokens: DefaultOverlapTokens,
		Separators:    append([]string(nil), DefaultSeparators...),
	}
}

func (o Options) normalized() Options {
	if o.MaxTokens <= 0 {
		o.MaxTokens = DefaultMaxTokens
	}
	if o.OverlapTokens < 0 {
		o.OverlapTokens = 0
	}
	if o.OverlapTokens >= o.MaxTokens {
		o.OverlapTokens = o.MaxTokens / 5
	}
	if len(o.Separators) == 0 {
		o.Separators = append([]string(nil), DefaultSeparators...)
	}
	return o
}

// Result 分块结果（尚未持久化）
type Result struct {
	Index      int
	Content    string
	ChunkHash  string
	TokenCount int
}
