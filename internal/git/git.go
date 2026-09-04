package git

import (
	"errors"
	"fmt"
	"os"
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

// LoadGitCommits prefers unpushed commits when the current branch has an
// upstream. When there are no unpushed commits (or no upstream is configured),
// it loads the full current-branch history so a synchronized branch never
// renders as an empty repository.
func LoadGitCommits() ([]*core.CommitState, error) {
	if !IsInsideGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	hasUpstream, err := currentBranchHasUpstream()
	if err != nil {
		return nil, err
	}
	if hasUpstream {
		commits, err := fetchGitLog("@{u}..HEAD")
		if err != nil {
			return nil, fmt.Errorf("load unpushed commits: %w", err)
		}
		if len(commits) > 0 {
			return commits, nil
		}
	}

	commits, err := fetchGitLog("HEAD")
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits found in repository")
	}

	return commits, nil
}

func currentBranchHasUpstream() (bool, error) {
	branchCmd := exec.Command("git", "symbolic-ref", "--quiet", "--short", "HEAD")
	if err := branchCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("detect current branch: %w", err)
	}

	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "@{upstream}^{commit}")
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("detect current branch upstream: %w", err)
	}
	return true, nil
}

// fetchGitLog executes git log and parses the custom delimited output.
// Delimiters: \x1f (Unit Separator between fields), \x1e (Record Separator between commits).
func fetchGitLog(args ...string) ([]*core.CommitState, error) {
	format := "%H%x1f%P%x1f%an%x1f%ae%x1f%aI%x1f%cn%x1f%ce%x1f%cI%x1f%G?%x1f%B%x1e"
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
		rec = strings.TrimPrefix(rec, "\n")
		if strings.TrimSpace(rec) == "" {
			continue
		}

		fields := strings.SplitN(rec, "\x1f", 10)
		if len(fields) < 10 {
			continue
		}

		hash := fields[0]
		parentStr := fields[1]
		authorName := fields[2]
		authorEmail := fields[3]
		dateStr := fields[4]
		committerName := fields[5]
		committerEmail := fields[6]
		committerDateStr := fields[7]
		signatureStatus := fields[8]
		message := fields[9]

		t, parseErr := time.Parse(time.RFC3339, dateStr)
		if parseErr != nil {
			// Fallback parsing for alternative git date formats
			t, _ = time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		}
		committerTime, parseErr := time.Parse(time.RFC3339, committerDateStr)
		if parseErr != nil {
			committerTime, _ = time.Parse("2006-01-02 15:04:05 -0700", committerDateStr)
		}

		var parents []string
		if strings.TrimSpace(parentStr) != "" {
			parents = strings.Fields(parentStr)
		}

		commits = append(commits, &core.CommitState{
			Hash:                  hash,
			OriginalHash:          hash,
			Message:               message,
			AuthorName:            authorName,
			AuthorEmail:           authorEmail,
			Timestamp:             t,
			OriginalTime:          t,
			OriginalName:          authorName,
			OriginalMail:          authorEmail,
			Selected:              false,
			CommitterName:         committerName,
			OriginalCommitterName: committerName,
			CommitterEmail:        committerEmail,
			OriginalCommitterMail: committerEmail,
			CommitterTime:         committerTime,
			OriginalCommitterTime: committerTime,
			IsSigned:              signatureStatus != "N",
			ParentHashes:          parents,
			IsMerge:               len(parents) > 1,
		})
	}

	return commits, nil
}

// CreateBackupBranch creates a backup branch before history rewrite.
func CreateBackupBranch() (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("toolgit-backup-%s", timestamp)
	sourceBranch, err := GetCurrentBranch()
	if err != nil {
		return "", fmt.Errorf("failed to identify source branch: %w", err)
	}

	cmd := exec.Command("git", "branch", branchName, "HEAD")
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create backup branch: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	configKey := fmt.Sprintf("branch.%s.toolgitSourceBranch", branchName)
	configCmd := exec.Command("git", "config", "--local", configKey, sourceBranch)
	if out, err := configCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to associate backup with branch %s: %s (%w)", sourceBranch, strings.TrimSpace(string(out)), err)
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
// Preserves merge topology by tracking old SHA → new SHA mappings.
// Commits must be passed in order from newest (HEAD) to oldest (git log order).
func ExecuteHistoryRewrite(commits []*core.CommitState) (string, error) {
	if len(commits) == 0 {
		return "", fmt.Errorf("no commits to rewrite")
	}
	for _, commit := range commits {
		if commit.IsSigned {
			return "", fmt.Errorf("commit %s is signed; refusing rewrite because its signature cannot be preserved", commit.OriginalHash)
		}
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

	// 3. Build the set of commit hashes being rewritten for parent resolution
	rewriteSet := make(map[string]bool, len(ordered))
	for _, c := range ordered {
		rewriteSet[c.OriginalHash] = true
	}

	// 4. Execute topology-preserving rewrite
	err = BatchRewriteHistory(ordered, rewriteSet)
	if err != nil {
		return backupBranch, fmt.Errorf("failed to batch rewrite history: %w", err)
	}

	return backupBranch, nil
}

// BatchRewriteHistory rewrites commit history while preserving merge topology
// via an old→new SHA mapping. Git is invoked directly so commit metadata is
// never interpreted by an intermediate command shell.
func BatchRewriteHistory(commits []*core.CommitState, rewriteSet map[string]bool) error {
	rewritten := make(map[string]string, len(commits))

	for _, c := range commits {
		treeCmd := exec.Command("git", "rev-parse", c.OriginalHash+"^{tree}")
		treeOut, err := treeCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("resolve tree for commit %s: %s (%w)", c.OriginalHash, strings.TrimSpace(string(treeOut)), err)
		}
		treeHash := strings.TrimSpace(string(treeOut))

		args := []string{"commit-tree", treeHash}
		for _, parentHash := range c.ParentHashes {
			resolvedParent := parentHash
			if rewriteSet[parentHash] {
				var ok bool
				resolvedParent, ok = rewritten[parentHash]
				if !ok {
					return fmt.Errorf("rewrite order invalid: parent %s of commit %s has not been rewritten", parentHash, c.OriginalHash)
				}
			}
			args = append(args, "-p", resolvedParent)
		}
		args = append(args, "-F", "-")

		isoDate := c.Timestamp.Format(time.RFC3339)
		committerName := c.CommitterName
		if committerName == "" {
			committerName = c.AuthorName
		}
		committerEmail := c.CommitterEmail
		if committerEmail == "" {
			committerEmail = c.AuthorEmail
		}
		committerTime := c.CommitterTime
		if committerTime.IsZero() {
			committerTime = c.Timestamp
		}
		commitCmd := exec.Command("git", args...)
		commitCmd.Stdin = strings.NewReader(c.Message)
		commitCmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME="+c.AuthorName,
			"GIT_AUTHOR_EMAIL="+c.AuthorEmail,
			"GIT_AUTHOR_DATE="+isoDate,
			"GIT_COMMITTER_NAME="+committerName,
			"GIT_COMMITTER_EMAIL="+committerEmail,
			"GIT_COMMITTER_DATE="+committerTime.Format(time.RFC3339),
		)

		commitOut, err := commitCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("rewrite commit %s: %s (%w)", c.OriginalHash, strings.TrimSpace(string(commitOut)), err)
		}
		newHash := strings.TrimSpace(string(commitOut))
		if newHash == "" {
			return fmt.Errorf("rewrite commit %s: git commit-tree returned an empty hash", c.OriginalHash)
		}
		rewritten[c.OriginalHash] = newHash
	}

	newHead := rewritten[commits[len(commits)-1].OriginalHash]
	updateCmd := exec.Command("git", "update-ref", "HEAD", newHead)
	if out, err := updateCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("update HEAD after rewrite: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// GetBackupBranches returns backup branches created for the current branch.
// Legacy unscoped backups are intentionally omitted because their source branch
// cannot be determined safely.
func GetBackupBranches() ([]string, error) {
	currentBranch, err := GetCurrentBranch()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("git", "branch", "--list", "toolgit-backup-*")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "* ")
		if line == "" {
			continue
		}

		configKey := fmt.Sprintf("branch.%s.toolgitSourceBranch", line)
		sourceOut, err := exec.Command("git", "config", "--local", "--get", configKey).Output()
		if err == nil && strings.TrimSpace(string(sourceOut)) == currentBranch {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// ExecuteRollback restores the current branch to one of its own toolgit backups.
// It refuses to run when the working tree or index is dirty.
func ExecuteRollback(branch string) error {
	if !strings.HasPrefix(branch, "toolgit-backup-") {
		return fmt.Errorf("refusing rollback to non-toolgit branch %q", branch)
	}

	currentBranch, err := GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("identify current branch: %w", err)
	}
	configKey := fmt.Sprintf("branch.%s.toolgitSourceBranch", branch)
	sourceOut, err := exec.Command("git", "config", "--local", "--get", configKey).CombinedOutput()
	if err != nil {
		return fmt.Errorf("backup %q has no trusted source-branch metadata", branch)
	}
	sourceBranch := strings.TrimSpace(string(sourceOut))
	if sourceBranch != currentBranch {
		return fmt.Errorf("backup %q belongs to branch %q, not current branch %q", branch, sourceBranch, currentBranch)
	}

	dirty, err := hasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("check working tree before rollback: %w", err)
	}
	if dirty {
		return fmt.Errorf("rollback refused: working tree or index has uncommitted changes")
	}

	verifyRef := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	if err := verifyRef.Run(); err != nil {
		return fmt.Errorf("backup branch %q does not exist", branch)
	}

	cmd := exec.Command("git", "reset", "--keep", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reset failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// GetLocalBranches returns all local branch names and the currently active branch.
func GetLocalBranches() ([]string, string, error) {
	cmd := exec.Command("git", "branch", "--list", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}

	var branches []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}

	current, err := GetCurrentBranch()
	if err != nil {
		current = ""
	}

	return branches, current, nil
}

// SwitchBranch checks out the specified local branch.
func SwitchBranch(branchName string) error {
	cmd := exec.Command("git", "checkout", branchName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("switch failed: %s (%w)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// HasUncommittedChanges returns true if the working tree or index has changes.
func HasUncommittedChanges() bool {
	dirty, err := hasUncommittedChanges()
	return err == nil && dirty
}

func hasUncommittedChanges() (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) != "", nil
}
