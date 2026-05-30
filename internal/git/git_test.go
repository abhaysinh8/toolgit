package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGitIntegrationAndRewrite(t *testing.T) {
	// Create a temp directory for git tests
	tempDir, err := os.MkdirTemp("", "toolgit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	runGit := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s (%v)", args, string(out), err)
		}
		return string(out)
	}

	// 1. Initialize repo
	runGit("init")
	runGit("config", "user.name", "Original Author")
	runGit("config", "user.email", "original@example.com")

	// 2. Create 3 test commits
	for i := 1; i <= 3; i++ {
		filePath := filepath.Join(tempDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("content "+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}
		runGit("add", "file.txt")
		runGit("commit", "-m", "Commit "+string(rune('0'+i)))
	}

	// Switch working dir temporarily for testing git loader & rewrite
	origWd, _ := os.Getwd()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer os.Chdir(origWd)

	// Verify repo detection
	if !IsInsideGitRepo() {
		t.Fatalf("expected to be inside git repo")
	}

	// Verify commit loading
	commits, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("failed to load commits: %v", err)
	}
	if len(commits) != 3 {
		t.Fatalf("expected 3 commits, got %d", len(commits))
	}

	// Modify commit #1 and #3 metadata
	commits[0].AuthorName = "Alice Modern"
	commits[0].AuthorEmail = "alice@modern.io"
	newTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	commits[0].Timestamp = newTime

	// Execute rewrite
	backupBranch, err := ExecuteHistoryRewrite(commits)
	if err != nil {
		t.Fatalf("history rewrite failed: %v", err)
	}
	if backupBranch == "" {
		t.Errorf("expected backup branch name, got empty")
	}

	// Verify backup branch exists
	branchesOut := runGit("branch", "--list", backupBranch)
	if branchesOut == "" {
		t.Errorf("expected backup branch %s to exist", backupBranch)
	}

	// Reload commits and verify new metadata
	reloaded, err := LoadGitCommits()
	if err != nil {
		t.Fatalf("failed to reload commits: %v", err)
	}
	if len(reloaded) != 3 {
		t.Fatalf("expected 3 reloaded commits, got %d", len(reloaded))
	}

	if reloaded[0].AuthorName != "Alice Modern" {
		t.Errorf("expected AuthorName 'Alice Modern', got %s", reloaded[0].AuthorName)
	}
	if reloaded[0].AuthorEmail != "alice@modern.io" {
		t.Errorf("expected AuthorEmail 'alice@modern.io', got %s", reloaded[0].AuthorEmail)
	}
}
