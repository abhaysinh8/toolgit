package main

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDistributeTimesOrganicMonotonicityAndBounds(t *testing.T) {
	start := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 27, 17, 0, 0, 0, time.UTC)

	commits := []*CommitState{
		{Hash: "1"},
		{Hash: "2"},
		{Hash: "3"},
		{Hash: "4"},
		{Hash: "5"},
	}

	DistributeTimes(commits, start, end)

	if !commits[len(commits)-1].Timestamp.Equal(start) && !commits[len(commits)-1].Timestamp.After(start) {
		t.Errorf("expected last (oldest) commit on or after start, got %v", commits[len(commits)-1].Timestamp)
	}
	if commits[0].Timestamp.After(end) {
		t.Errorf("expected first (newest) commit on or before end, got %v", commits[0].Timestamp)
	}

	// Verify strict descending order (newest-first)
	for i := 0; i < len(commits)-1; i++ {
		if !commits[i].Timestamp.After(commits[i+1].Timestamp) {
			t.Errorf("commit %d timestamp (%v) is not after commit %d timestamp (%v)",
				i, commits[i].Timestamp, i+1, commits[i+1].Timestamp)
		}
	}
}

func TestDistributeTimesMultiDayActiveDays(t *testing.T) {
	start := time.Date(2026, 4, 2, 5, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 24, 7, 0, 0, 0, time.UTC)

	// Create 22 commits
	commits := make([]*CommitState, 22)
	for i := 0; i < 22; i++ {
		commits[i] = &CommitState{Hash: string(rune('A' + i))}
	}

	// Distribute across at least 10 days
	DistributeTimesOrganic(commits, start, end, 10)

	// Collect unique calendar days
	uniqueDays := make(map[string]bool)
	for i, c := range commits {
		dayKey := c.Timestamp.Format("2006-01-02")
		uniqueDays[dayKey] = true

		if i > 0 && !commits[i-1].Timestamp.After(c.Timestamp) {
			t.Errorf("commit %d (%v) not after commit %d (%v)", i-1, commits[i-1].Timestamp, i, c.Timestamp)
		}
	}

	if len(uniqueDays) < 10 {
		t.Errorf("expected at least 10 unique active days, got %d (%v)", len(uniqueDays), uniqueDays)
	}
}

func TestDistributeTimesSingleItem(t *testing.T) {
	start := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 27, 17, 0, 0, 0, time.UTC)

	commits := []*CommitState{{Hash: "1"}}
	DistributeTimes(commits, start, end)

	if commits[0].Timestamp.Before(start) || commits[0].Timestamp.After(end) {
		t.Errorf("expected timestamp within [%v, %v], got %v", start, end, commits[0].Timestamp)
	}
}

func TestGenerateDryRunDiff(t *testing.T) {
	t1 := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

	commits := []*CommitState{
		{
			OriginalHash: "abc1234",
			OriginalName: "Old Author",
			AuthorName:   "New Author",
			OriginalMail: "old@example.com",
			AuthorEmail:  "old@example.com",
			OriginalTime: t1,
			Timestamp:    t1,
			Message:      "commit 1",
		},
		{
			OriginalHash: "def5678",
			OriginalName: "Same Author",
			AuthorName:   "Same Author",
			OriginalMail: "same@example.com",
			AuthorEmail:  "same@example.com",
			OriginalTime: t1,
			Timestamp:    t2, // modified time
			Message:      "commit 2",
		},
	}

	diffs := GenerateDryRunDiff(commits)
	if len(diffs) != 2 {
		t.Fatalf("expected 2 diff items, got %d", len(diffs))
	}

	if !diffs[0].IsModified || diffs[0].NewAuthor != "New Author" {
		t.Errorf("expected diff[0] to be modified with New Author, got %+v", diffs[0])
	}

	if !diffs[1].IsModified || !diffs[1].NewTime.Equal(t2) {
		t.Errorf("expected diff[1] to be modified with new time, got %+v", diffs[1])
	}
}

func TestTimePickerParsing(t *testing.T) {
	modal := NewTimePickerModal(2)
	modal.StartInput.SetValue("2026-08-27 09:00")
	modal.EndInput.SetValue("2026-08-27 17:00")
	modal.DaysInput.SetValue("1")

	st, et, minDays, err := modal.ParseTimes()
	if err != nil {
		t.Fatalf("unexpected error parsing valid times: %v", err)
	}
	if st.Hour() != 9 || et.Hour() != 17 || minDays != 1 {
		t.Errorf("expected 9:00 to 17:00 and 1 day, got %v to %v and %d days", st, et, minDays)
	}

	// Test invalid range
	modal.StartInput.SetValue("2026-08-27 18:00")
	modal.EndInput.SetValue("2026-08-27 17:00")
	_, _, _, err = modal.ParseTimes()
	if err == nil {
		t.Errorf("expected error when end is before start, got nil")
	}

	// Test invalid active days (exceeding calendar span)
	modal.StartInput.SetValue("2026-08-27 09:00")
	modal.EndInput.SetValue("2026-08-27 17:00")
	modal.DaysInput.SetValue("5") // only 1 calendar day available
	_, _, _, err = modal.ParseTimes()
	if err == nil {
		t.Errorf("expected error when active days exceed calendar span, got nil")
	}
}

func TestTimePickerDigitTyping(t *testing.T) {
	m := initialModel()
	// Open time picker modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(model)

	// Move to Custom Range (last preset)
	for i := 0; i < len(m.timeModal.Presets)-1; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = updated.(model)
	}

	// Press Enter to enter custom mode
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)

	if !m.timeModal.isCustomMode {
		t.Fatalf("expected custom mode to be active")
	}

	// Clear start input
	m.timeModal.StartInput.SetValue("")

	// Type digit '2'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(model)

	// Verify digit '2' is in start input
	if m.timeModal.StartInput.Value() != "2" {
		t.Errorf("expected start input value '2', got '%s'", m.timeModal.StartInput.Value())
	}
}
