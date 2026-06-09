package entity

type InterimChunk struct {
	FilePath   string
	Content    string
	ChunkIndex int
	LineStart  int
	LineEnd    int
}
