package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"toolgit/internal/core"
)

func setupTestRepo(t *testing.T) (string, func(args ...string) string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "toolgit-branch-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init", "-b", "main")
	runGit("config", "user.name", "Tester")
	runGit("config", "user.email", "tester@example.com")

	origWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}

	cleanup := func() {
		os.Chdir(origWd)
		os.RemoveAll(tempDir)
	}

	return tempDir, runGit, cleanup
}

func TestFetchGitLogParsesParents(t *testing.T) {
	tempDir, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	// 1. Commit 1 on main (root commit)
	os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("v1"), 0644)
	runGit("add", "file1.txt")
	runGit("commit", "-m", "Root commit")

	// 2. Branch feature and commit
	runGit("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(tempDir, "feature.txt"), []byte("feature work"), 0644)
	runGit("add", "feature.txt")
	runGit("commit", "-m", "Feature commit")

	// 3. Back to main and commit
	runGit("checkout", "main")
	os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("v2"), 0644)
	runGit("add", "file2.txt")
	runGit("commit", "-m", "Main commit")

	// 4. Merge feature into main (creates merge commit with 2 parents)
	runGit("merge", "--no-ff", "-m", "Merge branch feature", "feature")

	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("failed to load commits: %v", err)
	}

	if len(commits) < 4 {
		t.Fatalf("expected at least 4 commits, got %d", len(commits))
	}

	// Index 0: Merge commit
	mergeCommit := commits[0]
	if !mergeCommit.IsMerge {
		t.Errorf("expected commit 0 to be merge commit, got IsMerge = false")
	}
	if len(mergeCommit.ParentHashes) != 2 {
		t.Errorf("expected 2 parents for merge commit, got %d: %v", len(mergeCommit.ParentHashes), mergeCommit.ParentHashes)
	}

	// Root commit (last in list)
	rootCommit := commits[len(commits)-1]
	if rootCommit.IsMerge {
		t.Errorf("expected root commit not to be merge commit")
	}
	if len(rootCommit.ParentHashes) != 0 {
		t.Errorf("expected 0 parents for root commit, got %d: %v", len(rootCommit.ParentHashes), rootCommit.ParentHashes)
	}
}

func TestLoadGitCommitsFallsBackToHistoryWhenUpstreamIsCurrent(t *testing.T) {
	_, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	runGit("commit", "--allow-empty", "-m", "Initial")
	remoteDir := t.TempDir()
	runGit("init", "--bare", remoteDir)
	runGit("remote", "add", "origin", remoteDir)
	runGit("push", "-u", "origin", "main")

	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("load synchronized branch history: %v", err)
	}
	if len(commits) != 1 || strings.TrimSpace(commits[0].Message) != "Initial" {
		t.Fatalf("expected synchronized branch history, got %#v", commits)
	}

	runGit("commit", "--allow-empty", "-m", "Local work")
	commits, err = LoadGitCommits()
	if err != nil {
		t.Fatalf("load unpushed commit: %v", err)
	}
	if len(commits) != 1 || strings.TrimSpace(commits[0].Message) != "Local work" {
		t.Fatalf("expected only the unpushed commit, got %#v", commits)
	}
}

func TestLoadGitCommitsFromDetachedHead(t *testing.T) {
	_, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	runGit("commit", "--allow-empty", "-m", "Initial")
	runGit("commit", "--allow-empty", "-m", "Detached tip")
	runGit("checkout", "--detach", "HEAD")

	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("load detached HEAD: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("detached HEAD commit count = %d, want 2", len(commits))
	}
	if strings.TrimSpace(commits[0].Message) != "Detached tip" {
		t.Fatalf("detached HEAD tip = %q, want Detached tip", commits[0].Message)
	}
}

func TestBatchRewritePreservesMergeTopology(t *testing.T) {
	tempDir, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	// 1. Root commit
	os.WriteFile(filepath.Join(tempDir, "root.txt"), []byte("root"), 0644)
	runGit("add", "root.txt")
	runGit("commit", "-m", "Initial commit")

	// 2. Branch feature and commit
	runGit("checkout", "-b", "feature")
	os.WriteFile(filepath.Join(tempDir, "feature.txt"), []byte("feature"), 0644)
	runGit("add", "feature.txt")
	runGit("commit", "-m", "Branch feature commit")

	// 3. Checkout main and commit
	runGit("checkout", "main")
	os.WriteFile(filepath.Join(tempDir, "main.txt"), []byte("main"), 0644)
	runGit("add", "main.txt")
	runGit("commit", "-m", "Main branch commit")

	// 4. Merge feature into main
	runGit("merge", "--no-ff", "-m", "Merge feature into main", "feature")

	// 5. Load and rewrite all commits
	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("failed to load commits: %v", err)
	}

	// Change author on all commits
	for _, c := range commits {
		c.AuthorName = "New Author"
		c.AuthorEmail = "new@author.com"
		c.Timestamp = c.Timestamp.Add(24 * time.Hour)
	}

	backup, err := ExecuteHistoryRewrite(commits)
	if err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}
	if backup == "" {
		t.Fatalf("expected non-empty backup branch")
	}

	// 6. Inspect new HEAD commit
	parentsOut := runGit("rev-parse", "HEAD^@")
	parents := strings.Fields(parentsOut)
	if len(parents) != 2 {
		t.Fatalf("expected rewritten HEAD to have 2 parents, got %d (%s)", len(parents), parentsOut)
	}

	// Reload commits and verify metadata
	reloaded, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("failed to reload commits: %v", err)
	}

	if reloaded[0].AuthorName != "New Author" {
		t.Errorf("expected AuthorName 'New Author', got '%s'", reloaded[0].AuthorName)
	}
	if !reloaded[0].IsMerge {
		t.Errorf("expected reloaded commit 0 to be marked as merge commit")
	}
	if len(reloaded[0].ParentHashes) != 2 {
		t.Errorf("expected reloaded commit 0 to have 2 ParentHashes, got %d", len(reloaded[0].ParentHashes))
	}
}

func TestRewriteTreatsCommitMessageAsData(t *testing.T) {
	tempDir, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	markerPath := filepath.Join(tempDir, "injection-marker.txt")
	payload := "subject\n\"@\nSet-Content -LiteralPath '" + markerPath + "' -Value 'injected'\n$MSG = @\"\nbody"
	runGit("commit", "--allow-empty", "-m", payload)
	originalHead := runGit("rev-parse", "HEAD")

	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("failed to load commits: %v", err)
	}
	if _, err := ExecuteHistoryRewrite(commits); err != nil {
		t.Fatalf("rewrite failed: %v", err)
	}

	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("commit message was executed as code; marker stat error: %v", err)
	}

	message := runGit("log", "-1", "--format=%B")
	if !strings.Contains(message, "Set-Content -LiteralPath") {
		t.Fatalf("rewritten commit did not preserve the adversarial message: %q", message)
	}
	if rewrittenHead := runGit("rev-parse", "HEAD"); rewrittenHead != originalHead {
		t.Fatalf("no-op rewrite changed commit hash: got %s, want %s", rewrittenHead, originalHead)
	}
}

func TestRewritePreservesDistinctCommitterMetadata(t *testing.T) {
	tempDir, _, cleanup := setupTestRepo(t)
	defer cleanup()

	commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "Distinct identities")
	commitCmd.Dir = tempDir
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Author Person",
		"GIT_AUTHOR_EMAIL=author@example.com",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_NAME=Committer Person",
		"GIT_COMMITTER_EMAIL=committer@example.com",
		"GIT_COMMITTER_DATE=2026-02-03T04:05:06Z",
	)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("create commit with distinct committer: %s (%v)", out, err)
	}

	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("load commits: %v", err)
	}
	if len(commits) != 1 {
		t.Fatalf("expected one commit, got %d", len(commits))
	}
	commit := commits[0]
	if commit.CommitterName != "Committer Person" || commit.CommitterEmail != "committer@example.com" {
		t.Fatalf("committer metadata not loaded: %#v", commit)
	}
	originalCommitterTime := commit.CommitterTime
	commit.AuthorName = "Updated Author"

	if _, err := ExecuteHistoryRewrite(commits); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	reloaded, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("reload commits: %v", err)
	}
	got := reloaded[0]
	if got.AuthorName != "Updated Author" {
		t.Fatalf("author name = %q, want Updated Author", got.AuthorName)
	}
	if got.CommitterName != "Committer Person" || got.CommitterEmail != "committer@example.com" {
		t.Fatalf("committer metadata changed: %#v", got)
	}
	if !got.CommitterTime.Equal(originalCommitterTime) {
		t.Fatalf("committer time = %v, want %v", got.CommitterTime, originalCommitterTime)
	}
}

func TestRewriteRefusesSignedCommitBeforeCreatingBackup(t *testing.T) {
	_, err := ExecuteHistoryRewrite([]*core.CommitState{{
		OriginalHash: "signed-commit",
		IsSigned:     true,
	}})
	if err == nil || !strings.Contains(err.Error(), "signature cannot be preserved") {
		t.Fatalf("expected signed-commit refusal, got %v", err)
	}
}

func TestGetLocalBranchesAndSwitch(t *testing.T) {
	tempDir, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	os.WriteFile(filepath.Join(tempDir, "init.txt"), []byte("init"), 0644)
	runGit("add", "init.txt")
	runGit("commit", "-m", "Initial")

	runGit("branch", "alpha")
	runGit("branch", "beta")

	branches, current, err := GetLocalBranches()
	if err != nil {
		t.Fatalf("failed to get local branches: %v", err)
	}

	if current != "main" {
		t.Errorf("expected current branch 'main', got '%s'", current)
	}

	hasAlpha, hasBeta, hasMain := false, false, false
	for _, b := range branches {
		if b == "alpha" {
			hasAlpha = true
		}
		if b == "beta" {
			hasBeta = true
		}
		if b == "main" {
			hasMain = true
		}
	}

	if !hasAlpha || !hasBeta || !hasMain {
		t.Errorf("expected branches alpha, beta, main; got %v", branches)
	}

	// Switch branch
	if err := SwitchBranch("alpha"); err != nil {
		t.Fatalf("failed to switch branch: %v", err)
	}

	curAfter, err := GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if curAfter != "alpha" {
		t.Errorf("expected current branch 'alpha', got '%s'", curAfter)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	tempDir, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	os.WriteFile(filepath.Join(tempDir, "clean.txt"), []byte("clean"), 0644)
	runGit("add", "clean.txt")
	runGit("commit", "-m", "Initial")

	if HasUncommittedChanges() {
		t.Errorf("expected clean working tree, got dirty")
	}

	// Modify file
	os.WriteFile(filepath.Join(tempDir, "clean.txt"), []byte("modified dirty"), 0644)
	if !HasUncommittedChanges() {
		t.Errorf("expected dirty working tree after file modification")
	}

	// Stage it
	runGit("add", "clean.txt")
	if !HasUncommittedChanges() {
		t.Errorf("expected dirty working tree after staging")
	}

	// Commit it
	runGit("commit", "-m", "Update clean.txt")
	if HasUncommittedChanges() {
		t.Errorf("expected clean working tree after commit")
	}
}

func TestRollbackRefusesDirtyWorkingTree(t *testing.T) {
	tempDir, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	filePath := filepath.Join(tempDir, "tracked.txt")
	if err := os.WriteFile(filePath, []byte("original"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGit("add", "tracked.txt")
	runGit("commit", "-m", "Initial")

	backup, err := CreateBackupBranch()
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	originalHead := runGit("rev-parse", "HEAD")

	if err := os.WriteFile(filePath, []byte("local work"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := ExecuteRollback(backup); err == nil || !strings.Contains(err.Error(), "uncommitted changes") {
		t.Fatalf("expected dirty-worktree refusal, got %v", err)
	}

	if head := runGit("rev-parse", "HEAD"); head != originalHead {
		t.Fatalf("rollback moved HEAD despite dirty worktree: got %s, want %s", head, originalHead)
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read tracked file: %v", err)
	}
	if string(content) != "local work" {
		t.Fatalf("rollback changed local work: %q", content)
	}
}

func TestBackupsAreScopedToSourceBranch(t *testing.T) {
	_, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	runGit("commit", "--allow-empty", "-m", "Initial")
	backup, err := CreateBackupBranch()
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}

	backups, err := GetBackupBranches()
	if err != nil {
		t.Fatalf("list backups on source branch: %v", err)
	}
	if len(backups) != 1 || backups[0] != backup {
		t.Fatalf("expected source branch backup %q, got %v", backup, backups)
	}

	runGit("checkout", "-b", "feature")
	backups, err = GetBackupBranches()
	if err != nil {
		t.Fatalf("list backups on other branch: %v", err)
	}
	if len(backups) != 0 {
		t.Fatalf("expected no main backups on feature, got %v", backups)
	}

	if err := ExecuteRollback(backup); err == nil || !strings.Contains(err.Error(), "belongs to branch") {
		t.Fatalf("expected cross-branch rollback refusal, got %v", err)
	}
}

func TestRollbackRestoresCleanSourceBranch(t *testing.T) {
	_, runGit, cleanup := setupTestRepo(t)
	defer cleanup()

	runGit("commit", "--allow-empty", "-m", "Initial")
	backup, err := CreateBackupBranch()
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	wantHead := runGit("rev-parse", backup)
	runGit("commit", "--allow-empty", "-m", "Later")

	if err := ExecuteRollback(backup); err != nil {
		t.Fatalf("rollback clean branch: %v", err)
	}
	if head := runGit("rev-parse", "HEAD"); head != wantHead {
		t.Fatalf("rollback HEAD = %s, want %s", head, wantHead)
	}
}
