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
