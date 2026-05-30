package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"toolgit/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

// TestUserJourneySimulation simulates a complete interactive session of a user using toolgit.
func TestUserJourneySimulation(t *testing.T) {
	// 1. Initialize Model
	m := initialModel()
	// Force mock mode so we don't accidentally rewrite the real toolgit repo during testing
	m.isRealRepo = false
	if len(m.commits) == 0 {
		t.Fatalf("expected initial commits to be populated")
	}

	// 2. Test Navigation
	// 1. Initial State Check
	if m.table.Cursor() != 0 {
		t.Errorf("Expected initial cursor to be 0, got %d", m.table.Cursor())
	}
	// Press 'j' to move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(model)
	if m.table.Cursor() != 1 {
		t.Errorf("expected cursor to be 1, got %d", m.table.Cursor())
	}

	// Press 'k' to move up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(model)
	if m.table.Cursor() != 0 {
		t.Errorf("Expected cursor to move back to 0, got %d", m.table.Cursor())
	}

	// 3. Test Selection (Space)
	if m.commits[0].Selected {
		t.Errorf("expected initial commit not to be selected")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(model)
	if !m.commits[0].Selected {
		t.Errorf("expected commit 0 to be selected after space")
	}

	// 4. Test Select All ('a')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(model)
	for i, c := range m.commits {
		if !c.Selected {
			t.Errorf("expected commit %d to be selected after 'a'", i)
		}
	}

	// 5. Test Author / Email Modal Editor ('e')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(model)
	if m.activeModal != ModalEditAuthor {
		t.Fatalf("expected ModalEditAuthor to be active, got %v", m.activeModal)
	}

	// Set new author name and email
	m.authorModal.NameInput.SetValue("Jane Developer")
	m.authorModal.EmailInput.SetValue("jane@developer.org")

	// Press Enter to apply modal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.activeModal != ModalNone {
		t.Errorf("expected modal to close, got %v", m.activeModal)
	}

	// Verify all selected commits updated
	for i, c := range m.commits {
		if c.AuthorName != "Jane Developer" || c.AuthorEmail != "jane@developer.org" {
			t.Errorf("commit %d author not updated properly: name=%s, email=%s", i, c.AuthorName, c.AuthorEmail)
		}
	}

	// 6. Test Custom Time Range Modal ('d')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(model)
	if m.activeModal != ModalTimeShift {
		t.Errorf("Expected TimePicker modal to be active")
	}

	// Apply Preset 0 (Today 9:00 - 17:00) with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.activeModal != ModalNone {
		t.Errorf("expected time modal to close, got %v", m.activeModal)
	}

	// Verify timestamps are distributed in newest-first order
	for i := 0; i < len(m.commits)-1; i++ {
		if !m.commits[i].Timestamp.After(m.commits[i+1].Timestamp) {
			t.Errorf("commit %d timestamp (%v) is not after commit %d timestamp (%v)",
				i, m.commits[i].Timestamp, i+1, m.commits[i+1].Timestamp)
		}
	}

	// 7. Test Dry-Run Modal ('w')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(model)
	if m.activeModal != ModalDryRun {
		t.Fatalf("expected ModalDryRun to be active, got %v", m.activeModal)
	}

	// Confirm Dry-Run ('y')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)
	if m.activeModal != ModalDryRun || m.dryRunModal.State != DryRunStateDone {
		t.Errorf("expected dry run modal to transition to Done, got %v (state %v)", m.activeModal, m.dryRunModal.State)
	}

	// Close Done screen
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.activeModal != ModalNone {
		t.Errorf("expected dry run modal to close after done, got %v", m.activeModal)
	}
	if !strings.Contains(m.status, "Applied changes") && !strings.Contains(m.status, "Rewrote history") {
		t.Errorf("expected success status message, got: %s", m.status)
	}

	// 8. Test View Rendering
	m.width = 120
	m.height = 30
	viewOutput := m.View()
	if !strings.Contains(viewOutput, "toolgit") {
		t.Errorf("expected rendered view to contain 'toolgit', got: %s", viewOutput)
	}
	if !strings.Contains(viewOutput, "Jane Developer") {
		t.Errorf("expected rendered view to contain 'Jane Developer', got: %s", viewOutput)
	}

	// 9. Test Process Safety / Clean Quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Errorf("expected Ctrl+C to return tea.Quit command")
	}
}

func TestRealRepoUserWorkflow(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "toolgit-e2e-*")
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

	// Create test git repo with commits
	runGit("init")
	runGit("config", "user.name", "Dev Original")
	runGit("config", "user.email", "dev@orig.org")

	for i := 1; i <= 4; i++ {
		file := filepath.Join(tempDir, "code.go")
		os.WriteFile(file, []byte(string(rune('A'+i))), 0644)
		runGit("add", "code.go")
		runGit("commit", "-m", "Step "+string(rune('0'+i)))
	}

	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Launch initial model inside real repository
	m := initialModel()
	if !m.isRealRepo {
		t.Fatalf("expected real repo to be detected")
	}
	if len(m.commits) != 4 {
		t.Fatalf("expected 4 commits loaded from real repo, got %d", len(m.commits))
	}

	// Select all and batch edit
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m.commits[0].AuthorName = "Dev Modified"
	m.commits[0].AuthorEmail = "dev@modified.org"

	// Open Dry-Run and Execute History Rewrite
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	m = updated.(model)
	if m.activeModal != ModalDryRun {
		t.Fatalf("expected DryRun modal")
	}

	// Confirm rewrite
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)

	if m.dryRunModal.State != DryRunStateExecuting {
		t.Fatalf("expected Executing state, got %v", m.dryRunModal.State)
	}

	// Manually execute the rewrite since we are bypassing the Bubble Tea event loop in tests
	backup, err := git.ExecuteHistoryRewrite(m.commits)

	// Feed the finish message back into the model
	updated, _ = m.Update(RewriteFinishedMsg{BackupBranch: backup, Err: err})
	m = updated.(model)

	if m.dryRunModal.State != DryRunStateDone {
		t.Fatalf("expected Done state, got %v", m.dryRunModal.State)
	}

	// Close Done screen
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if !strings.Contains(m.status, "Rewrote history") {
		t.Fatalf("expected history rewrite message, got %s", m.status)
	}

	// Verify git log contains new author
	logOut := runGit("log", "-1", "--format=%an <%ae>")
	if !strings.Contains(logOut, "Dev Modified <dev@modified.org>") {
		t.Errorf("expected git log to reflect 'Dev Modified', got: %s", logOut)
	}
}
