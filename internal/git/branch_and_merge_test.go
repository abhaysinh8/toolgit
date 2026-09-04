package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
