package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"toolgit/internal/core"
)

// IsInsideGitRepo checks if the current working directory is inside a Git repository.
func IsInsideGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

// GetCurrentBranch returns the name of the currently checked-out branch.
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// LoadGitCommits attempts to load commits from the repository.
// It prioritizes unpushed commits (@{u}..HEAD); if no upstream is configured,
// it loads all commits on the current branch.
func LoadGitCommits() ([]*core.CommitState, error) {
	if !IsInsideGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	// Try unpushed commits first
	commits, err := fetchGitLog("@{u}..HEAD")
	if err == nil && len(commits) > 0 {
		return commits, nil
	}

	// Fallback to all local branch commits
	commits, err = fetchGitLog("HEAD")
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits found in repository")
	}

	return commits, nil
}

// fetchGitLog executes git log and parses the custom delimited output.
// Delimiters: \x1f (Unit Separator between fields), \x1e (Record Separator between commits).
func fetchGitLog(args ...string) ([]*core.CommitState, error) {
	format := "%H%x1f%an%x1f%ae%x1f%aI%x1f%B%x1e"
	cmdArgs := append([]string{"log", fmt.Sprintf("--format=%s", format)}, args...)
	cmd := exec.Command("git", cmdArgs...)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := string(out)
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	records := strings.Split(raw, "\x1e")
	var commits []*core.CommitState

	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}

		fields := strings.Split(rec, "\x1f")
		if len(fields) < 5 {
			continue
		}

		hash := fields[0]
		authorName := fields[1]
		authorEmail := fields[2]
		dateStr := fields[3]
		message := fields[4]

		t, parseErr := time.Parse(time.RFC3339, dateStr)
		if parseErr != nil {
			// Fallback parsing for alternative git date formats
			t, _ = time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		}

		commits = append(commits, &core.CommitState{
			Hash:         hash,
			OriginalHash: hash,
			Message:      message,
			AuthorName:   authorName,
			AuthorEmail:  authorEmail,
			Timestamp:    t,
			OriginalTime: t,
			OriginalName: authorName,
			OriginalMail: authorEmail,
			Selected:     false,
		})
	}

	return commits, nil
}

// CreateBackupBranch creates a backup branch before history rewrite.
func CreateBackupBranch() (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("toolgit-backup-%s", timestamp)

	cmd := exec.Command("git", "branch", branchName, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create backup branch: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return branchName, nil
}

// GenerateDryRunDiff compares original commit states against current edits.
func GenerateDryRunDiff(commits []*core.CommitState) []core.DiffItem {
	var diffs []core.DiffItem
	for _, c := range commits {
		modified := c.AuthorName != c.OriginalName ||
			c.AuthorEmail != c.OriginalMail ||
			!c.Timestamp.Equal(c.OriginalTime)

		diffs = append(diffs, core.DiffItem{
			OldHash:    c.OriginalHash,
			NewHash:    "pending rewrite",
			OldAuthor:  c.OriginalName,
			NewAuthor:  c.AuthorName,
			OldEmail:   c.OriginalMail,
			NewEmail:   c.AuthorEmail,
			OldTime:    c.OriginalTime,
			NewTime:    c.Timestamp,
			Message:    c.Message,
			IsModified: modified,
		})
	}
	return diffs
}

// ExecuteHistoryRewrite safely rewrites commit history using Git plumbing commands.
// Commits must be passed in order (HEAD down to oldest or oldest to HEAD).
func ExecuteHistoryRewrite(commits []*core.CommitState) (string, error) {
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits to rewrite")
	}

	// 1. Create a safety backup branch
	backupBranch, err := CreateBackupBranch()
	if err != nil {
		return "", fmt.Errorf("pre-rewrite backup failed: %w", err)
	}

	// 2. Commits are displayed newest first in Git log.
	// We reverse them to rewrite from oldest ancestor to newest (HEAD).
	ordered := make([]*core.CommitState, len(commits))
	for i := range commits {
		ordered[i] = commits[len(commits)-1-i]
	}

	// Find the parent of the oldest commit in our chain
	oldestHash := ordered[0].OriginalHash
	parentCmd := exec.Command("git", "rev-parse", oldestHash+"^@")
	parentOut, _ := parentCmd.Output()

	// Handle multiple parents (e.g. merge commits) by grabbing the first parent,
	// or format it cleanly for the PowerShell script.
	parents := strings.Fields(strings.TrimSpace(string(parentOut)))
	var baseUpstreamSHA string
	if len(parents) > 0 {
		baseUpstreamSHA = parents[0] // Just take the primary parent for the rewritten base
	}

	// Execute O(1) process rewrite via PowerShell
	err = BatchRewriteHistory(ordered, baseUpstreamSHA)
	if err != nil {
		return backupBranch, fmt.Errorf("failed to batch rewrite history: %w", err)
	}

	// We don't get the exact new hash of every commit back from the batch script,
	// but we can refresh the commit list from disk after the rewrite completes.
	return backupBranch, nil
}

func escapePS(val string) string {
	return strings.ReplaceAll(val, "'", "''")
}

func BatchRewriteHistory(commits []*core.CommitState, baseUpstreamSHA string) error {
	var script strings.Builder

	// 1. Initialize $PARENT
	script.WriteString(fmt.Sprintf("$PARENT = '%s'\n", escapePS(baseUpstreamSHA)))

	// 2. Loop through commits
	for _, c := range commits {
		authorName := escapePS(c.AuthorName)
		authorEmail := escapePS(c.AuthorEmail)
		isoDate := c.Timestamp.Format(time.RFC3339)

		script.WriteString(fmt.Sprintf("$env:GIT_AUTHOR_NAME='%s'\n", authorName))
		script.WriteString(fmt.Sprintf("$env:GIT_AUTHOR_EMAIL='%s'\n", authorEmail))
		script.WriteString(fmt.Sprintf("$env:GIT_AUTHOR_DATE='%s'\n", isoDate))
		script.WriteString(fmt.Sprintf("$env:GIT_COMMITTER_NAME='%s'\n", authorName))
		script.WriteString(fmt.Sprintf("$env:GIT_COMMITTER_EMAIL='%s'\n", authorEmail))
		script.WriteString(fmt.Sprintf("$env:GIT_COMMITTER_DATE='%s'\n", isoDate))

		// Escape backticks and dollar signs for double-quoted Here-String
		msg := strings.ReplaceAll(c.Message, "`", "``")
		msg = strings.ReplaceAll(msg, "$", "`$")

		script.WriteString(fmt.Sprintf("$MSG = @\"\n%s\n\"@\n", msg))

		// Extract Tree SHA via PowerShell
		script.WriteString(fmt.Sprintf("$TREE = git rev-parse \"%s^{tree}\"\n", c.OriginalHash))

		// Run commit-tree and capture into $PARENT
		script.WriteString("if ([string]::IsNullOrWhiteSpace($PARENT)) {\n")
		script.WriteString("    $PARENT = git commit-tree $TREE -m $MSG\n")
		script.WriteString("} else {\n")
		script.WriteString("    $PARENT = git commit-tree $TREE -p $PARENT -m $MSG\n")
		script.WriteString("}\n")
		script.WriteString("if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\n\n")
	}

	// 3. Atomically update the branch HEAD
	script.WriteString("git update-ref HEAD $PARENT\n")
	script.WriteString("if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }\n")

	// 4. Execute the script via Stdin
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "-")
	cmd.Stdin = strings.NewReader(script.String())

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("batch rewrite failed: %w\nStderr: %s", err, stderr.String())
	}

	return nil
}

// GetBackupBranches returns a list of all backup branches created by toolgit.
func GetBackupBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--list", "toolgit-backup-*")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ExecuteRollback hard resets the current branch to the specified backup branch.
func ExecuteRollback(branch string) error {
	cmd := exec.Command("git", "reset", "--hard", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reset failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}
