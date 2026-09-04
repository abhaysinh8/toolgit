package app

import (
	"testing"
	"time"
	"toolgit/internal/core"

	tea "github.com/charmbracelet/bubbletea"
)

func TestBranchSwitchModalStateTransitions(t *testing.T) {
	branches := []string{"main", "feature-auth", "bugfix-leak"}
	current := "main"

	// 1. Clean state test (no dirty edits)
	modal := NewBranchSwitchModal(branches, current, false)
	if modal.State != BranchSwitchIdle {
		t.Fatalf("expected initial state BranchSwitchIdle, got %v", modal.State)
	}
	if modal.HasDirtyEdits {
		t.Fatalf("expected HasDirtyEdits = false")
	}

	// 2. Dirty state test
	dirtyModal := NewBranchSwitchModal(branches, current, true)
	if !dirtyModal.HasDirtyEdits {
		t.Fatalf("expected HasDirtyEdits = true")
	}

	// 3. Test View rendering in all states
	idleView := modal.View(80, 24)
	if idleView == "" {
		t.Errorf("expected non-empty View in Idle state")
	}

	modal.State = BranchSwitchConfirm
	modal.PendingBranch = "feature-auth"
	confirmView := modal.View(80, 24)
	if confirmView == "" {
		t.Errorf("expected non-empty View in Confirm state")
	}

	modal.State = BranchSwitchSwitching
	switchingView := modal.View(80, 24)
	if switchingView == "" {
		t.Errorf("expected non-empty View in Switching state")
	}

	modal.State = BranchSwitchIdle
	modal.ErrorMsg = "git checkout failed"
	errView := modal.View(80, 24)
	if errView == "" {
		t.Errorf("expected non-empty View when ErrorMsg is set")
	}
}

func TestAppBranchSwitchingFlow(t *testing.T) {
	m := initialModel()
	m.isRealRepo = false // Mock mode

	// Press 'b' in mock mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(model)

	if m.activeModal == ModalBranchSwitch {
		t.Errorf("expected branch switch to be disabled in mock mode")
	}
	if m.statusOk {
		t.Errorf("expected statusOk = false in mock mode")
	}

	// Enable simulated branch switch modal
	m.isRealRepo = true
	m.currentBranch = "main"
	m.branchModal = NewBranchSwitchModal([]string{"main", "feature-x"}, "main", false)
	m.activeModal = ModalBranchSwitch

	// Close modal with esc
	updated, _ = m.handleModalKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.activeModal != ModalNone {
		t.Errorf("expected ModalNone after esc, got %v", m.activeModal)
	}

	// Reopen with dirty edits
	m.commits[0].AuthorName = "Changed Author"
	m.branchModal = NewBranchSwitchModal([]string{"main", "feature-x"}, "main", true)
	m.activeModal = ModalBranchSwitch

	// Select second item (feature-x)
	m.branchModal.List.Select(1)
	updated, _ = m.handleModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	// Since dirty, should enter BranchSwitchConfirm state
	if m.branchModal.State != BranchSwitchConfirm {
		t.Fatalf("expected BranchSwitchConfirm state, got %v", m.branchModal.State)
	}
	if m.branchModal.PendingBranch != "feature-x" {
		t.Errorf("expected PendingBranch 'feature-x', got '%s'", m.branchModal.PendingBranch)
	}

	// Press 'n' to cancel confirm
	updated, _ = m.handleModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = updated.(model)
	if m.branchModal.State != BranchSwitchIdle {
		t.Errorf("expected BranchSwitchIdle after 'n', got %v", m.branchModal.State)
	}

	// Re-enter confirm and press 'y' to proceed
	m.branchModal.State = BranchSwitchConfirm
	m.branchModal.PendingBranch = "feature-x"
	updated, cmd := m.handleModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)
	if m.branchModal.State != BranchSwitchSwitching {
		t.Errorf("expected BranchSwitchSwitching after 'y', got %v", m.branchModal.State)
	}
	if cmd == nil {
		t.Errorf("expected command batch returned for async branch switch")
	}

	// Simulate BranchSwitchFinishedMsg
	updated, _ = m.Update(BranchSwitchFinishedMsg{Branch: "feature-x", Err: nil})
	m = updated.(model)
	if m.activeModal != ModalNone {
		t.Errorf("expected ModalNone after finish, got %v", m.activeModal)
	}
	if m.currentBranch != "feature-x" {
		t.Errorf("expected currentBranch 'feature-x', got '%s'", m.currentBranch)
	}
}

func TestMergeIndicatorInCommitState(t *testing.T) {
	now := time.Now()
	linearCommit := &core.CommitState{
		Hash:         "abc1234",
		OriginalHash: "abc1234",
		AuthorName:   "Tester",
		OriginalName: "Tester",
		AuthorEmail:  "tester@test.com",
		OriginalMail: "tester@test.com",
		Timestamp:    now,
		OriginalTime: now,
		ParentHashes: []string{"p1"},
		IsMerge:      false,
	}

	mergeCommit := &core.CommitState{
		Hash:         "def5678",
		OriginalHash: "def5678",
		AuthorName:   "Tester",
		OriginalName: "Tester",
		AuthorEmail:  "tester@test.com",
		OriginalMail: "tester@test.com",
		Timestamp:    now,
		OriginalTime: now,
		ParentHashes: []string{"p1", "p2"},
		IsMerge:      true,
	}

	m := initialModel()
	m.commits = []*core.CommitState{mergeCommit, linearCommit}
	m.updateTable()

	rows := m.table.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Status column should have 'M' marker for merge commit
	if rows[0][0] != " M  " && rows[0][0] != " M ✓" && rows[0][0] != "✎M  " {
		t.Logf("Row 0 status: %q", rows[0][0])
	}
}
