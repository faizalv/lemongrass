package entity

type FileDiff struct {
	FilePath     string
	Diff         string
	IsNew        bool
	LinesAdded   int
	LinesRemoved int
}
