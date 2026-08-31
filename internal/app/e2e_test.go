package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"toolgit/internal/git"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestTUIDimensionsAndZeroScrolling(t *testing.T) {
	sizes := [][2]int{
		{50, 15}, // Extreme zoom in
		{65, 20}, // Zoomed in
		{80, 24}, // Standard terminal
		{100, 30}, // Medium terminal
		{120, 40}, // Large terminal
		{160, 50}, // Zoomed out
	}
	modals := []ActiveModal{
		ModalNone,
		ModalEditAuthor,
		ModalTimeShift,
		ModalDryRun,
		ModalRollback,
	}

	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		for _, mod := range modals {
			m := initialModel()
			// Simulate dynamic window resize / zoom event
			newM, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
			m = newM.(model)
			m.activeModal = mod
			if mod == ModalEditAuthor {
				m.authorModal = NewEditAuthorModal("Test Author", "test@example.com", 1, false)
			} else if mod == ModalTimeShift {
				m.timeModal = NewTimePickerModal(1)
			} else if mod == ModalDryRun {
				m.dryRunModal = DryRunModal{
					Diffs:      git.GenerateDryRunDiff(m.commits),
					IsRealRepo: false,
				}
			} else if mod == ModalRollback {
				m.rollbackModal = NewRollbackModal([]string{"toolgit-backup-test"})
			}

			// Test across multiple commit cursor positions (scrolling up/down)
			for cursor := 0; cursor < len(m.commits); cursor++ {
				m.table.SetCursor(cursor)
				view := m.View()
				lines := strings.Split(view, "\n")
				if len(lines) > h {
					t.Fatalf("Size %dx%d Cursor %d Modal %v rendered %d lines (expected <= %d)", w, h, cursor, mod, len(lines), h)
				}
				for i, line := range lines {
					lineWidth := lipgloss.Width(line)
					if lineWidth > w {
						t.Fatalf("Size %dx%d Cursor %d Modal %v Line %d width %d exceeds terminal width %d:\n%s", w, h, cursor, mod, i, lineWidth, w, line)
					}
				}
				if mod != ModalNone {
					break // Modals don't need cursor iteration
				}
			}
		}
	}
}

func TestLipglossChrome(t *testing.T) {
	w, h := 80, 24
	leftOuterWidth := (w * 44) / 100 // 35
	rightOuterWidth := w - leftOuterWidth // 45
	paneInnerHeight := h - 6 // 18

	leftStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(leftOuterWidth - 2).
		Height(paneInnerHeight)

	rightStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(rightOuterWidth - 2).
		Height(paneInnerHeight)

	lp := leftStyle.Render("Left Content")
	rp := rightStyle.Render("Right Content")

	t.Logf("LeftPane outer width: %d, height: %d", lipgloss.Width(lp), lipgloss.Height(lp))
	t.Logf("RightPane outer width: %d, height: %d", lipgloss.Width(rp), lipgloss.Height(rp))

	body := lipgloss.JoinHorizontal(lipgloss.Top, lp, rp)
	t.Logf("Body outer width: %d, height: %d", lipgloss.Width(body), lipgloss.Height(body))

	if lipgloss.Width(body) != w {
		t.Fatalf("Expected body width %d, got %d", w, lipgloss.Width(body))
	}
	if lipgloss.Height(body) != h-4 {
		t.Fatalf("Expected body height %d, got %d", h-4, lipgloss.Height(body))
	}
}

func TestScrollUpDownStepByStep(t *testing.T) {
	sizes := [][2]int{
		{80, 24},
		{100, 30},
		{60, 18},
	}

	for _, sz := range sizes {
		w, h := sz[0], sz[1]
		m := initialModel()
		newM, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
		m = newM.(model)

		// Send Down key 30 times
		downKey := tea.KeyMsg{Type: tea.KeyDown}
		for step := 0; step < 30; step++ {
			newM, _ = m.Update(downKey)
			m = newM.(model)

			view := m.View()
			lines := strings.Split(view, "\n")
			if len(lines) != h {
				for idx, l := range lines {
					t.Logf("[%02d] (len %d): %s", idx, lipgloss.Width(l), l)
				}
				t.Fatalf("Size %dx%d Step Down %d: line count is %d, expected exactly %d", w, h, step, len(lines), h)
			}
			for i, line := range lines {
				lw := lipgloss.Width(line)
				if lw > w {
					t.Fatalf("Size %dx%d Step Down %d Line %d: width is %d, expected <= %d:\n%s", w, h, step, i, lw, w, line)
				}
			}
		}

		// Send Up key 30 times
		upKey := tea.KeyMsg{Type: tea.KeyUp}
		for step := 0; step < 30; step++ {
			newM, _ = m.Update(upKey)
			m = newM.(model)

			view := m.View()
			lines := strings.Split(view, "\n")
			if len(lines) != h {
				t.Fatalf("Size %dx%d Step Up %d: line count is %d, expected exactly %d", w, h, step, len(lines), h)
			}
			for i, line := range lines {
				lw := lipgloss.Width(line)
				if lw > w {
					t.Fatalf("Size %dx%d Step Up %d Line %d: width is %d, expected <= %d:\n%s", w, h, step, i, lw, w, line)
				}
			}
		}
	}
}
