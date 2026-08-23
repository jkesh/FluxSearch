package settings

const (
	DocumentDedupModeSkip    = "skip"
	DocumentDedupModeReplace = "replace"

	ChunkDedupScopeDocument    = "document"
	ChunkDedupScopeCollection = "collection"
)

type DedupSettings struct {
	DocumentDedupEnabled       bool   `json:"document_dedup_enabled"`
	DocumentDedupMode          string `json:"document_dedup_mode"`
	DocumentDedupByContentHash bool   `json:"document_dedup_by_content_hash"`
	DocumentDedupBySourceURI   bool   `json:"document_dedup_by_source_uri"`
	ChunkDedupEnabled          bool   `json:"chunk_dedup_enabled"`
	ChunkDedupScope            string `json:"chunk_dedup_scope"`
}

func defaultDedupSettings() DedupSettings {
	return DedupSettings{
		DocumentDedupEnabled:       true,
		DocumentDedupMode:          DocumentDedupModeSkip,
		DocumentDedupByContentHash: true,
		DocumentDedupBySourceURI:   true,
		ChunkDedupEnabled:          false,
		ChunkDedupScope:            ChunkDedupScopeCollection,
	}
}

func (d DedupSettings) Normalized() DedupSettings {
	out := d
	if out.DocumentDedupMode != DocumentDedupModeReplace {
		out.DocumentDedupMode = DocumentDedupModeSkip
	}
	if out.ChunkDedupScope != ChunkDedupScopeDocument {
		out.ChunkDedupScope = ChunkDedupScopeCollection
	}
	return out
}

func (s AppSettings) withDedupDefaults() AppSettings {
	def := defaultDedupSettings()
	if s.DocumentDedupMode == "" {
		s.DocumentDedupMode = def.DocumentDedupMode
	}
	if s.ChunkDedupScope == "" {
		s.ChunkDedupScope = def.ChunkDedupScope
	}
	return s
}
