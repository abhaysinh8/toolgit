package core

import "time"

type CommitState struct {
	Hash         string
	OriginalHash string
	Message      string
	AuthorName   string
	OriginalName string
	AuthorEmail  string
	OriginalMail string
	Timestamp    time.Time
	OriginalTime time.Time
	Selected     bool
	ParentHashes []string // All parent SHAs (len > 1 indicates merge commit)
	IsMerge      bool     // Convenience flag for merge commits
}

type DiffItem struct {
	OldHash     string
	NewHash     string
	OldAuthor   string
	NewAuthor   string
	OldEmail    string
	NewEmail    string
	OldTime     time.Time
	NewTime     time.Time
	Message     string
	IsModified  bool
}
