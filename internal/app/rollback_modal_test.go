package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRollbackRequiresConfirmation(t *testing.T) {
	m := newMockModelForTest()
	m.activeModal = ModalRollback
	m.rollbackModal = NewRollbackModal([]string{"toolgit-backup-test"})

	updated, cmd := m.handleModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if cmd != nil {
		t.Fatal("selecting a backup should not execute rollback immediately")
	}
	if m.rollbackModal.State != RollbackStateConfirm {
		t.Fatalf("state = %v, want confirmation", m.rollbackModal.State)
	}
	if m.rollbackModal.PendingBranch != "toolgit-backup-test" {
		t.Fatalf("pending branch = %q", m.rollbackModal.PendingBranch)
	}
	if view := m.rollbackModal.View(80); !strings.Contains(view, "Confirm Rollback") {
		t.Fatalf("confirmation view missing warning: %q", view)
	}

	updated, cmd = m.handleModalKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if cmd != nil || m.rollbackModal.State != RollbackStateSelect {
		t.Fatalf("cancel should return to selection, state=%v cmd=%v", m.rollbackModal.State, cmd)
	}

	updated, _ = m.handleModalKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	updated, cmd = m.handleModalKeys(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = updated.(model)
	if cmd == nil {
		t.Fatal("confirming rollback should return an execution command")
	}
	if m.rollbackModal.State != RollbackStateExecuting {
		t.Fatalf("state = %v, want executing", m.rollbackModal.State)
	}
}
