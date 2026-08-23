package chunker

// DefaultMaxChars 约 512 tokens（中英文混合粗估 chars/4）
const DefaultMaxChars = 2048

// DefaultOverlap 约 64 tokens
const DefaultOverlap = 256

// DefaultSeparators 递归分割优先级：段落 → 行 → 中英文句号 → 空格 → 硬切
var DefaultSeparators = []string{"\n\n", "\n", "。", ".", " ", ""}

type Options struct {
	MaxChars    int
	Overlap     int
	Separators  []string
}

func DefaultOptions() Options {
	return Options{
		MaxChars:   DefaultMaxChars,
		Overlap:    DefaultOverlap,
		Separators: append([]string(nil), DefaultSeparators...),
	}
}

func (o Options) normalized() Options {
	if o.MaxChars <= 0 {
		o.MaxChars = DefaultMaxChars
	}
	if o.Overlap < 0 {
		o.Overlap = 0
	}
	if o.Overlap >= o.MaxChars {
		o.Overlap = o.MaxChars / 5
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
