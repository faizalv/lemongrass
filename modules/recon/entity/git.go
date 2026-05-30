package entity

type GitStatus struct {
	IsGitRepo     bool
	Branch        string
	HeadCommit    string
	HeadMessage   string
	ChangedFiles  []ChangedFile
	StaleCount    int
	RecentCommits []CommitInfo
}

type ChangedFile struct {
	Path   string
	Status string // "modified" | "added" | "deleted"
}

type CommitInfo struct {
	Hash      string
	Message   string
	Author    string
	Timestamp string
}
