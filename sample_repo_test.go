package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestComprehensiveRealRepoWorkflow creates a real git repository with 5 commits,
// runs a complete interactive toolgit session (author edit, dropdown time distribution,
// dry-run review, and history rewrite), and asserts all git log outputs and backup branches.
func TestComprehensiveRealRepoWorkflow(t *testing.T) {
	// 1. Setup a dedicated temporary repository
	tempDir, err := os.MkdirTemp("", "toolgit-live-demo-*")
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

	runGit("init")
	runGit("config", "user.name", "Legacy Author")
	runGit("config", "user.email", "legacy@company.old")

	// Create 5 commits with initial timestamps
	commitMessages := []string{
		"feat(api): initial auth handler setup",
		"fix(db): resolve connection pool leak",
		"refactor(models): standardize json tags",
		"docs: add swagger API annotations",
		"test: add integration test suite",
	}

	for i, msg := range commitMessages {
		fileName := fmt.Sprintf("file_%d.txt", i+1)
		filePath := filepath.Join(tempDir, fileName)
		os.WriteFile(filePath, fmt.Appendf(nil, "Content for step %d\n", i+1), 0644)
		runGit("add", fileName)
		runGit("commit", "-m", msg)
	}

	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Verify before state
	beforeLog := runGit("log", "--oneline")
	t.Logf("\n--- INITIAL GIT LOG ---\n%s", beforeLog)

	// 2. Initialize Model in the repository
	m := initialModel()
	if !m.isRealRepo {
		t.Fatalf("expected real repo to be detected")
	}
	if len(m.commits) != 5 {
		t.Fatalf("expected 5 commits, got %d", len(m.commits))
	}

	// 3. Select all commits with 'a'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(model)

	// 4. Open Author Modal with 'e' and change to 'Grace Hopper <grace@navy.mil>'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	if m.activeModal != ModalEditAuthor {
		t.Fatalf("expected ModalEditAuthor, got %v", m.activeModal)
	}

	m.authorModal.NameInput.SetValue("Grace Hopper")
	m.authorModal.EmailInput.SetValue("grace@navy.mil")
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	// 5. Open Dropdown Time Picker with 'd' and select Preset 1 (Yesterday 9am-5pm)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(model)
	if m.activeModal != ModalTimePicker {
		t.Fatalf("expected ModalTimePicker, got %v", m.activeModal)
	}

	// Move to Preset 1 (Yesterday Workday) using 'j'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(model)
	if m.timeModal.selectedPreset != 1 {
		t.Fatalf("expected selected preset 1, got %d", m.timeModal.selectedPreset)
	}

	// Apply Preset 1 with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.activeModal != ModalNone {
		t.Fatalf("expected modal to close after applying preset")
	}

	// 6. Open Dry-Run Review with 'w'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(model)
	if m.activeModal != ModalDryRun {
		t.Fatalf("expected ModalDryRun, got %v", m.activeModal)
	}

	// Verify dry run diffs
	if len(m.dryRunModal.Diffs) != 5 {
		t.Fatalf("expected 5 diffs, got %d", len(m.dryRunModal.Diffs))
	}
	for i, d := range m.dryRunModal.Diffs {
		if !d.IsModified {
			t.Errorf("expected commit %d to be marked as modified", i)
		}
		if d.NewAuthor != "Grace Hopper" || d.NewEmail != "grace@navy.mil" {
			t.Errorf("commit %d diff new author mismatch: %s <%s>", i, d.NewAuthor, d.NewEmail)
		}
	}

	// 7. Confirm History Rewrite with 'y'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)

	if !strings.Contains(m.status, "Rewrote history") {
		t.Fatalf("expected status message 'Rewrote history', got: %s", m.status)
	}

	// 8. Assert Post-Rewrite Git State
	afterLog := runGit("log", "--format=%h | %an <%ae> | %ad | %s", "--date=format:%Y-%m-%d %H:%M:%S")
	t.Logf("\n--- REWRITTEN GIT LOG ---\n%s", afterLog)

	lines := strings.Split(strings.TrimSpace(afterLog), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines in rewritten log, got %d", len(lines))
	}

	for i, line := range lines {
		if !strings.Contains(line, "Grace Hopper <grace@navy.mil>") {
			t.Errorf("line %d did not contain new author: %s", i, line)
		}
	}

	// 9. Verify backup branch exists
	branches := runGit("branch", "--list", "toolgit-backup-*")
	if strings.TrimSpace(branches) == "" {
		t.Errorf("expected backup branch to be created, got none")
	}
	t.Logf("Backup branches created: %s", strings.TrimSpace(branches))

	// 10. Verify working directory is clean
	status := runGit("status", "--porcelain")
	if strings.TrimSpace(status) != "" {
		t.Errorf("expected clean working directory, got: %s", status)
	}
}
