package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CommitState represents a Git commit's metadata and selection status.
type CommitState struct {
	Hash         string
	OriginalHash string
	Message      string
	AuthorName   string
	OriginalName string
	AuthorEmail  string
	OriginalMail string
	Timestamp    time.Time
	OriginalTime time.Time
	Selected     bool
}

type RewriteFinishedMsg struct {
	BackupBranch string
	Err          error
}

type RollbackFinishedMsg struct {
	Branch string
	Err    error
}

// DistributeTimes spaces commits naturally with realistic human jitter across the time window.
func DistributeTimes(commits []*CommitState, start time.Time, end time.Time) {
	DistributeTimesOrganic(commits, start, end, 0)
}

// DistributeTimesOrganic distributes commits with organic human jitter, natural seconds, and active days partitioning.
func DistributeTimesOrganic(commits []*CommitState, start time.Time, end time.Time, minDays int) {
	n := len(commits)
	if n == 0 {
		return
	}
	if n == 1 {
		commits[0].Timestamp = start
		return
	}
	if end.Before(start) {
		end = start
	}

	// Git log returns commits newest-first. We need to assign times 
	// chronologically, so we reverse the slice to process oldest-first.
	chronoCommits := make([]*CommitState, n)
	for i := 0; i < n; i++ {
		chronoCommits[i] = commits[n-1-i]
	}

	// Calculate calendar span
	sDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	eDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	calendarDays := int(eDay.Sub(sDay).Hours()/24) + 1
	if calendarDays < 1 {
		calendarDays = 1
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// 1. Single Day Window (or sub-day range)
	if calendarDays <= 1 {
		distributeSingleDayOrganic(chronoCommits, start, end, rng)
		return
	}

	// 2. Multi-Day Distribution across active days
	targetDays := minDays
	if targetDays <= 0 {
		targetDays = int(math.Ceil(float64(n) / 2.5))
	}
	if targetDays > calendarDays {
		targetDays = calendarDays
	}
	if targetDays > n {
		targetDays = n
	}
	if targetDays < 1 {
		targetDays = 1
	}

	// Select day indices from 0 to calendarDays-1
	var chosenDays []int
	switch targetDays {
	case calendarDays:
		for d := 0; d < calendarDays; d++ {
			chosenDays = append(chosenDays, d)
		}
	case 1:
		chosenDays = []int{0}
	default:
		// Always include first day (0) and last day (calendarDays-1)
		chosenDays = append(chosenDays, 0)
		middleCount := targetDays - 2
		if middleCount > 0 && calendarDays > 2 {
			available := make([]int, 0, calendarDays-2)
			for d := 1; d < calendarDays-1; d++ {
				available = append(available, d)
			}
			rng.Shuffle(len(available), func(i, j int) {
				available[i], available[j] = available[j], available[i]
			})
			for i := 0; i < middleCount && i < len(available); i++ {
				chosenDays = append(chosenDays, available[i])
			}
		}
		chosenDays = append(chosenDays, calendarDays-1)
		sort.Ints(chosenDays)
	}

	// Partition n commits across len(chosenDays)
	dayCount := len(chosenDays)
	commitsPerDay := make([]int, dayCount)
	for i := 0; i < dayCount; i++ {
		commitsPerDay[i] = 1
	}
	remaining := n - dayCount
	for i := 0; i < remaining; i++ {
		commitsPerDay[rng.Intn(dayCount)]++
	}

	commitIdx := 0
	for j, dayOffset := range chosenDays {
		numForDay := commitsPerDay[j]
		curDate := sDay.AddDate(0, 0, dayOffset)

		dayStart := time.Date(curDate.Year(), curDate.Month(), curDate.Day(), 9, 30, 0, 0, curDate.Location())
		dayEnd := time.Date(curDate.Year(), curDate.Month(), curDate.Day(), 18, 30, 0, 0, curDate.Location())

		if dayOffset == 0 && start.After(dayStart) {
			dayStart = start
		}
		if dayOffset == calendarDays-1 && end.Before(dayEnd) {
			dayEnd = end
		}
		if !dayEnd.After(dayStart) {
			dayEnd = dayStart.Add(30 * time.Minute)
		}

		daySlice := chronoCommits[commitIdx : commitIdx+numForDay]
		distributeSingleDayOrganic(daySlice, dayStart, dayEnd, rng)
		commitIdx += numForDay
	}
}

func distributeSingleDayOrganic(commits []*CommitState, start, end time.Time, rng *rand.Rand) {
	n := len(commits)
	if n == 0 {
		return
	}
	if n == 1 {
		dur := end.Sub(start)
		if dur > 0 {
			jitter := time.Duration(rng.Float64() * float64(dur))
			commits[0].Timestamp = start.Add(jitter)
		} else {
			commits[0].Timestamp = start
		}
		return
	}

	totalDur := end.Sub(start)
	if totalDur <= 0 {
		for _, c := range commits {
			c.Timestamp = start
		}
		return
	}

	// Generate normalized random weights
	weights := make([]float64, n-1)
	sumW := 0.0
	for i := 0; i < n-1; i++ {
		w := 0.5 + rng.Float64()
		weights[i] = w
		sumW += w
	}

	cur := start
	commits[0].Timestamp = cur
	for i := 1; i < n; i++ {
		stepFrac := weights[i-1] / sumW
		stepDur := time.Duration(stepFrac * float64(totalDur))
		secJitter := time.Duration(rng.Intn(30)-15) * time.Second
		cur = cur.Add(stepDur + secJitter)
		prev := commits[i-1].Timestamp
		if !cur.After(prev.Add(5 * time.Second)) {
			cur = prev.Add(30 * time.Second)
		}
		if cur.After(end) {
			cur = end
		}
		commits[i].Timestamp = cur
	}
	if commits[n-1].Timestamp.After(end) {
		commits[n-1].Timestamp = end
	}
}

// generateMockCommits generates 5 mock commits when no Git repository is found.
func generateMockCommits() []*CommitState {
	now := time.Now()
	mockData := []struct {
		hash, msg, name, email string
		offset                 time.Duration
	}{
		{"7f3a9b2c8e4d1f0a5b6c7d8e9f0a1b2c3d4e5f6a", "feat(tui): initialize split-pane layout and themes", "Alex Chen", "alex.chen@example.com", -4 * time.Hour},
		{"3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e", "fix(git): resolve author timestamp parsing on Windows", "Alex Chen", "alex.chen@example.com", -3 * time.Hour},
		{"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", "refactor(core): optimize git rebase batch processing", "Sarah Connor", "s.connor@cyberdyne.io", -2 * time.Hour},
		{"e9f8a7b6c5d4e3f2a1b0c9d8e7f6a5b4c3d2e1f0", "docs: update installation instructions for CLI", "John Doe", "johndoe@users.noreply.github.com", -1 * time.Hour},
		{"5c4b3a2f1e0d9c8b7a6f5e4d3c2b1a0f9e8d7c6b", "test: add unit tests for time distribution algorithm", "Alex Chen", "alex.chen@example.com", 0},
	}

	var list []*CommitState
	for _, m := range mockData {
		t := now.Add(m.offset)
		list = append(list, &CommitState{
			Hash:         m.hash,
			OriginalHash: m.hash,
			Message:      m.msg,
			AuthorName:   m.name,
			OriginalName: m.name,
			AuthorEmail:  m.email,
			OriginalMail: m.email,
			Timestamp:    t,
			OriginalTime: t,
			Selected:     false,
		})
	}
	return list
}

// Styling definitions
var (
	subtleColor    = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#71717A"}
	highlightColor = lipgloss.AdaptiveColor{Light: "#6366F1", Dark: "#818CF8"}
	accentColor    = lipgloss.AdaptiveColor{Light: "#10B981", Dark: "#34D399"}
	warnColor      = lipgloss.AdaptiveColor{Light: "#EF4444", Dark: "#F87171"}
	textColor      = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F3F4F6"}

	titleBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#6366F1")).
			Padding(0, 1)

	branchBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#10B981")).
			Padding(0, 1)

	mockBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1E1E2E")).
			Background(lipgloss.Color("#F59E0B")).
			Padding(0, 1)

	activePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(highlightColor).
				Padding(0, 1)

	inactivePaneBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#3F3F46")).
				Padding(0, 1)

	cursorItemStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#4F46E5")).
			Padding(0, 1)

	normalItemStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	hashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FBBF24")).
			Bold(true)

	detailKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#A1A1AA")).
			Width(14)

	detailValStyle = lipgloss.NewStyle().
			Foreground(textColor)

	detailHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(highlightColor)

	keyBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#E0E7FF")).
			Background(lipgloss.Color("#3730A3")).
			Padding(0, 1)

	keyDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF"))
)

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Select    key.Binding
	SelectAll key.Binding
	Edit      key.Binding
	Time      key.Binding
	Apply     key.Binding
	Rollback  key.Binding
	Quit      key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Edit, k.Time, k.Apply, k.Rollback, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.SelectAll},
		{k.Edit, k.Time, k.Apply, k.Rollback, k.Quit},
	}
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Select:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select")),
	SelectAll: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all")),
	Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit author")),
	Time:      key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "distribute times")),
	Apply:     key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "apply/dry-run")),
	Rollback:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rollback")),
	Quit:      key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
}

type model struct {
	commits     []*CommitState
	table       table.Model
	help        help.Model
	keys        keyMap
	width       int
	height      int
	status      string
	statusOk    bool
	isRealRepo  bool
	currentBranch string

	// Modals
	activeModal   ActiveModal
	authorModal   EditAuthorModal
	timeModal     TimePickerModal
	dryRunModal   DryRunModal
	rollbackModal RollbackModal
}

func initialModel() model {
	var commits []*CommitState
	isReal := false
	branch := ""

	if IsInsideGitRepo() {
		realCommits, err := LoadGitCommits()
		if err == nil && len(realCommits) > 0 {
			commits = realCommits
			isReal = true
			branch, _ = GetCurrentBranch()
		}
	}

	if len(commits) == 0 {
		commits = generateMockCommits()
	}

	// Initialize Help
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().
		Background(lipgloss.Color("#4F46E5")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(0, 1).
		MarginRight(1).
		Bold(true)
	h.Styles.ShortDesc = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#A1A1AA"))
	h.Styles.ShortSeparator = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#27272A")).
		Padding(0, 1)

	// Initialize Table
	columns := []table.Column{
		{Title: " ", Width: 4}, // Status (Modified, Selected)
		{Title: "Hash", Width: 8},
		{Title: "Message", Width: 50},
	}
	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)
	
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	statusMsg := "Ready."
	if !isReal {
		statusMsg = "Mock Mode (No Git repo found). Changes will be simulated."
	}

	m := model{
		commits:       commits,
		table:         t,
		help:          h,
		keys:          keys,
		status:        statusMsg,
		statusOk:      true,
		isRealRepo:    isReal,
		currentBranch: branch,
		activeModal:   ModalNone,
	}
	m.updateTable()
	return m
}

func (m *model) updateTable() {
	var rows []table.Row
	for _, c := range m.commits {
		shortHash := c.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		check := " "
		if c.Selected {
			check = "✓"
		}
		modMarker := " "
		if c.AuthorName != c.OriginalName || c.AuthorEmail != c.OriginalMail || !c.Timestamp.Equal(c.OriginalTime) {
			modMarker = "✎"
		}
		status := fmt.Sprintf("%s %s", modMarker, check)

		msg := c.Message
		if len(msg) > 50 {
			msg = msg[:47] + "..."
		}
		rows = append(rows, table.Row{status, shortHash, msg})
	}
	m.table.SetRows(rows)
}

func (m model) Init() tea.Cmd {
	return nil
}

// getTargetCommits returns selected commits, or if none selected, the currently hovered commit.
func (m model) getTargetCommits() ([]*CommitState, bool) {
	var selected []*CommitState
	for _, c := range m.commits {
		if c.Selected {
			selected = append(selected, c)
		}
	}
	if len(selected) > 0 {
		return selected, true
	}
	cursor := m.table.Cursor()
	if len(m.commits) > 0 && cursor < len(m.commits) {
		return []*CommitState{m.commits[cursor]}, false
	}
	return nil, false
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case spinner.TickMsg:
		if m.activeModal == ModalDryRun {
			var cmd tea.Cmd
			m.dryRunModal.Spinner, cmd = m.dryRunModal.Spinner.Update(msg)
			return m, cmd
		} else if m.activeModal == ModalRollback && m.rollbackModal.State == RollbackStateExecuting {
			var cmd tea.Cmd
			m.rollbackModal.Spinner, cmd = m.rollbackModal.Spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case RewriteFinishedMsg:
		if m.activeModal == ModalDryRun {
			if msg.Err != nil {
				m.dryRunModal.State = DryRunStateReview
				m.dryRunModal.ErrorMsg = msg.Err.Error()
			} else {
				m.dryRunModal.State = DryRunStateDone
				m.dryRunModal.BackupBranch = msg.BackupBranch
				m.dryRunModal.TargetBranch = m.currentBranch
				
				// Count selected commits for the summary
				targets, _ := m.getTargetCommits()
				m.dryRunModal.Rewritten = len(targets)
				m.status = fmt.Sprintf("🚀 Rewrote history! Backup branch: %s", msg.BackupBranch)
				m.statusOk = true
			}
		}
		return m, nil

	case RollbackFinishedMsg:
		if m.activeModal == ModalRollback {
			if msg.Err != nil {
				m.status = "Rollback failed: " + msg.Err.Error()
				m.statusOk = false
			} else {
				m.status = "Successfully rolled back to " + msg.Branch
				m.statusOk = true
				// reload commits
				if newCommits, err := LoadGitCommits(); err == nil {
					m.commits = newCommits
					m.updateTable()
				}
			}
			m.activeModal = ModalNone
		}
		return m, nil

	case tea.KeyMsg:
		// Strict process safety: catch KeyCtrlC and cleanly quit
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}

		// Handle Modal-specific key events first
		if m.activeModal != ModalNone {
			return m.handleModalKeys(msg)
		}

		// Main View Keybindings
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up), key.Matches(msg, m.keys.Down):
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		case key.Matches(msg, m.keys.Select):
			cursor := m.table.Cursor()
			if len(m.commits) > 0 && cursor < len(m.commits) {
				m.commits[cursor].Selected = !m.commits[cursor].Selected
				m.status = fmt.Sprintf("Toggled commit %s", m.commits[cursor].Hash[:7])
				m.statusOk = true
				m.updateTable()
			}
		case key.Matches(msg, m.keys.SelectAll):
			// Toggle select all
			allSelected := true
			for _, c := range m.commits {
				if !c.Selected {
					allSelected = false
					break
				}
			}
			for _, c := range m.commits {
				c.Selected = !allSelected
			}
			if !allSelected {
				m.status = fmt.Sprintf("Selected all %d commits", len(m.commits))
			} else {
				m.status = "Deselected all commits"
			}
			m.statusOk = true
			m.updateTable()
		case key.Matches(msg, m.keys.Edit):
			targets, isBatch := m.getTargetCommits()
			if len(targets) == 0 {
				m.status = "No commits available to edit."
				m.statusOk = false
				return m, nil
			}
			defaultName := targets[0].AuthorName
			defaultEmail := targets[0].AuthorEmail
			m.authorModal = NewEditAuthorModal(defaultName, defaultEmail, len(targets), isBatch)
			m.activeModal = ModalEditAuthor
		case key.Matches(msg, m.keys.Time):
			targets, _ := m.getTargetCommits()
			if len(targets) == 0 {
				m.status = "No commits available for time distribution."
				m.statusOk = false
				return m, nil
			}
			m.timeModal = NewTimePickerModal(len(targets))
			m.activeModal = ModalTimeShift
		case key.Matches(msg, m.keys.Apply):
			// Open Dry-Run / Rewrite Modal
			diffs := GenerateDryRunDiff(m.commits)
			s := spinner.New()
			s.Spinner = spinner.Dot
			s.Style = lipgloss.NewStyle().Foreground(accentColor)
			
			m.dryRunModal = DryRunModal{
				State:      DryRunStateReview,
				Spinner:    s,
				Diffs:      diffs,
				IsRealRepo: m.isRealRepo,
			}
			m.activeModal = ModalDryRun
		case key.Matches(msg, m.keys.Rollback):
			backups, err := GetBackupBranches()
			if err != nil || len(backups) == 0 {
				m.status = "No backup branches found for rollback."
				m.statusOk = false
				return m, nil
			}
			m.rollbackModal = NewRollbackModal(backups)
			m.activeModal = ModalRollback
		}
	}

	return m, nil
}

func (m model) handleModalKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeModal {
	case ModalEditAuthor:
		switch msg.String() {
		case "esc":
			m.activeModal = ModalNone
			return m, nil

		case "tab", "down":
			m.authorModal.NextField()
			return m, nil

		case "shift+tab", "up":
			m.authorModal.PrevField()
			return m, nil

		case "enter":
			newName := m.authorModal.NameInput.Value()
			newEmail := m.authorModal.EmailInput.Value()

			targets, _ := m.getTargetCommits()
			for _, c := range targets {
				if newName != "" {
					c.AuthorName = newName
				}
				if newEmail != "" {
					c.AuthorEmail = newEmail
				}
			}
			m.activeModal = ModalNone
			m.status = fmt.Sprintf("✓ Updated author details for %d commit(s)", len(targets))
			m.statusOk = true
			return m, nil

		default:
			var cmd tea.Cmd
			if m.authorModal.focusIndex == 0 {
				m.authorModal.NameInput, cmd = m.authorModal.NameInput.Update(msg)
			} else {
				m.authorModal.EmailInput, cmd = m.authorModal.EmailInput.Update(msg)
			}
			return m, cmd
		}

	case ModalTimeShift:
		if !m.timeModal.isCustomMode {
			// Preset List Navigation Mode
			switch msg.String() {
			case "esc", "q":
				m.activeModal = ModalNone
				return m, nil

			case "up", "k":
				m.timeModal.PrevPreset()
				return m, nil

			case "down", "j":
				m.timeModal.NextPreset()
				return m, nil

			case "tab":
				m.timeModal.NextPreset()
				return m, nil

			case "enter":
				// If on Custom Range, switch to editing inputs
				if m.timeModal.selectedPreset == len(m.timeModal.Presets)-1 {
					m.timeModal.ActivateCustomMode()
					return m, nil
				}

				// Otherwise, apply highlighted preset immediately
				st, et, _, err := m.timeModal.ParseTimes()
				if err != nil {
					m.timeModal.errMsg = err.Error()
					return m, nil
				}

				targets, _ := m.getTargetCommits()
				DistributeTimesOrganic(targets, st, et, 0)
				m.activeModal = ModalNone
				m.status = fmt.Sprintf("✓ Distributed timestamps for %d commit(s) (%s)",
					len(targets), m.timeModal.Presets[m.timeModal.selectedPreset].Label)
				m.statusOk = true
				return m, nil
			}
		} else {
			// Custom Inputs Editing Mode
			switch msg.String() {
			case "esc":
				m.timeModal.SwitchToPresets()
				return m, nil

			case "tab":
				m.timeModal.NextCustomField()
				return m, nil

			case "enter":
				st, et, minDays, err := m.timeModal.ParseTimes()
				if err != nil {
					m.timeModal.errMsg = err.Error()
					return m, nil
				}

				targets, _ := m.getTargetCommits()
				DistributeTimesOrganic(targets, st, et, minDays)
				m.activeModal = ModalNone
				m.status = fmt.Sprintf("✓ Distributed %d commit(s) from %s to %s with organic jitter",
					len(targets), st.Format("Jan 02 15:04"), et.Format("Jan 02 15:04"))
				m.statusOk = true
				return m, nil

			default:
				var cmd tea.Cmd
				switch m.timeModal.customFocus {
				case 0:
					m.timeModal.StartInput, cmd = m.timeModal.StartInput.Update(msg)
				case 1:
					m.timeModal.EndInput, cmd = m.timeModal.EndInput.Update(msg)
				case 2:
					m.timeModal.DaysInput, cmd = m.timeModal.DaysInput.Update(msg)
				}
				return m, cmd
			}
		}

	case ModalRollback:
		if m.rollbackModal.State == RollbackStateExecuting {
			return m, nil
		}

		switch msg.String() {
		case "esc", "q":
			m.activeModal = ModalNone
			return m, nil
		case "enter":
			selected := m.rollbackModal.List.SelectedItem()
			if selected != nil {
				branch := selected.FilterValue()
				m.rollbackModal.State = RollbackStateExecuting
				return m, tea.Batch(
					m.rollbackModal.Spinner.Tick,
					func() tea.Msg {
						err := ExecuteRollback(branch)
						return RollbackFinishedMsg{Branch: branch, Err: err}
					},
				)
			}
			m.activeModal = ModalNone
			return m, nil
		default:
			var cmd tea.Cmd
			m.rollbackModal.List, cmd = m.rollbackModal.List.Update(msg)
			return m, cmd
		}

	case ModalDryRun:
		switch m.dryRunModal.State {
		case DryRunStateReview:
			switch msg.String() {
			case "esc", "n", "N":
				m.activeModal = ModalNone
				return m, nil
			case "enter", "y", "Y":
				if m.isRealRepo {
					m.dryRunModal.State = DryRunStateExecuting
					
					// Capture commits slice to pass to goroutine
					commitsToRewrite := m.commits
					
					return m, tea.Batch(
						m.dryRunModal.Spinner.Tick,
						func() tea.Msg {
							backup, err := ExecuteHistoryRewrite(commitsToRewrite)
							return RewriteFinishedMsg{BackupBranch: backup, Err: err}
						},
					)
				} else {
					// Mock mode in-memory sync
					for _, c := range m.commits {
						c.OriginalName = c.AuthorName
						c.OriginalMail = c.AuthorEmail
						c.OriginalTime = c.Timestamp
						c.Selected = false
					}
					m.dryRunModal.State = DryRunStateDone
					m.dryRunModal.BackupBranch = "mock-backup-branch"
					m.dryRunModal.TargetBranch = m.currentBranch
					targets, _ := m.getTargetCommits()
					m.dryRunModal.Rewritten = len(targets)
					m.status = "✓ [Mock Mode] Applied changes in memory successfully."
					m.statusOk = true
				}
				return m, nil
			}
		case DryRunStateExecuting:
			// Ignore all input while executing
			return m, nil
		case DryRunStateDone:
			switch msg.String() {
			case "enter", "esc", "q", " ":
				m.activeModal = ModalNone
				
				if m.isRealRepo {
					newCommits, err := LoadGitCommits()
					if err == nil {
						m.commits = newCommits
						m.updateTable()
					}
				}
				return m, nil
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 {
		m.width = 100
	}
	if m.height == 0 {
		m.height = 30
	}

	// Layout dimensions (reduced by 1 line to prevent Windows scrolling)
	headerHeight := 1
	footerHeight := 3
	paneHeight := m.height - headerHeight - footerHeight - 1
	if paneHeight < 8 {
		paneHeight = 8
	}

	// Balanced proportional split: Commit list gets ~48% width, details get ~52%
	leftWidth := (m.width * 48) / 100
	if leftWidth < 38 {
		leftWidth = 38
	}
	rightWidth := m.width - leftWidth - 2
	if rightWidth < 36 {
		rightWidth = 36
	}

	// 1. Header Bar
	var repoBadge string
	if m.isRealRepo {
		branchName := m.currentBranch
		if branchName == "" {
			branchName = "HEAD"
		}
		repoBadge = branchBadge.Render(" 🌿 " + branchName + " ")
	} else {
		repoBadge = mockBadge.Render(" 🧪 Mock Mode ")
	}

	headerLeft := titleBadge.Render(" ❖ toolgit ") + " " +
		lipgloss.NewStyle().Foreground(subtleColor).Render("Safe Git Commit Metadata Editor")

	headerGap := m.width - lipgloss.Width(headerLeft) - lipgloss.Width(repoBadge) - 2
	if headerGap < 1 {
		headerGap = 1
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerLeft,
		strings.Repeat(" ", headerGap),
		repoBadge,
	)

	// 2. Left Pane (Commit List with table)
	// table.Height sets the number of data rows. 
	// The table also renders a header (1 line) and a header bottom border (1 line).
	// To fit within activePaneBorder (which adds 2 lines of border), 
	// the table data rows should be paneHeight - 4.
	tableHeight := paneHeight - 4
	if tableHeight < 1 {
		tableHeight = 1
	}
	m.table.SetHeight(tableHeight)
	m.table.SetWidth(leftWidth - 2)

	listContent := m.table.View()
	leftPane := activePaneBorder.
		Width(leftWidth).
		Height(paneHeight).
		Render(listContent)

	// 3. Right Pane (Commit Details)
	var rightPaneContent string
	if len(m.commits) > 0 && m.table.Cursor() < len(m.commits) {
		cur := m.commits[m.table.Cursor()]

		isModified := cur.AuthorName != cur.OriginalName ||
			cur.AuthorEmail != cur.OriginalMail ||
			!cur.Timestamp.Equal(cur.OriginalTime)

		var statusBadge string
		if isModified {
			statusBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#DB2777")).
				Padding(0, 1).
				Render("✎ MODIFIED")
		} else {
			statusBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#10B981")).
				Render("● Unchanged")
		}

		var selBadge string
		if cur.Selected {
			selBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#059669")).
				Padding(0, 1).
				Render("✓ Selected")
		} else {
			selBadge = lipgloss.NewStyle().
				Foreground(subtleColor).
				Render("○ Not Selected")
		}

		shortHash := cur.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		detailHeader := lipgloss.JoinHorizontal(
			lipgloss.Center,
			detailHeaderStyle.Render("Commit "+shortHash),
			"  ",
			statusBadge,
			"  ",
			selBadge,
		)

		msgBoxWidth := rightWidth - 6
		if msgBoxWidth < 20 {
			msgBoxWidth = 20
		}

		msgBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(highlightColor).
			Padding(0, 1).
			Width(msgBoxWidth).
			Foreground(textColor).
			Render(cur.Message)

		details := []string{
			detailHeader,
			"",
			renderDetailRow("Full SHA:", hashStyle.Render(cur.Hash)),
			renderDetailRow("Author:", detailValStyle.Render(cur.AuthorName)),
			renderDetailRow("Email:", detailValStyle.Render(cur.AuthorEmail)),
			renderDetailRow("Date:", detailValStyle.Render(cur.Timestamp.Format("Mon Jan 02, 2006 • 15:04:05 MST"))),
			renderDetailRow("Relative:", detailValStyle.Render(formatRelativeTime(cur.Timestamp))),
			"",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A1A1AA")).Render("Message:"),
			msgBox,
		}
		rightPaneContent = lipgloss.JoinVertical(lipgloss.Left, details...)
	} else {
		rightPaneContent = detailValStyle.Render("No commit selected.")
	}

	rightPane := inactivePaneBorder.
		Width(rightWidth).
		Height(paneHeight).
		Render(rightPaneContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	// 4. Footer: Status Line + Help Key Badges
	statusIcon := lipgloss.NewStyle().Foreground(accentColor).Render("●")
	if !m.statusOk {
		statusIcon = lipgloss.NewStyle().Foreground(warnColor).Render("▲")
	}

	var selectedCount int
	for _, c := range m.commits {
		if c.Selected {
			selectedCount++
		}
	}
	countsInfo := lipgloss.NewStyle().Foreground(subtleColor).Render(
		fmt.Sprintf("[%d/%d]  [%d selected]", m.table.Cursor()+1, len(m.commits), selectedCount),
	)

	statusLine := fmt.Sprintf("%s %s  %s", statusIcon, m.status, countsInfo)

	// Help Bar matching original clean style
	helpKeyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Bold(true)
	helpDescStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
	
	renderBtn := func(key, desc string) string {
		return fmt.Sprintf("%s %s", helpKeyStyle.Render(key), helpDescStyle.Render(desc))
	}
	
	helpBar := lipgloss.JoinHorizontal(lipgloss.Left,
		renderBtn("space", "select"), " • ",
		renderBtn("a", "select all"), " • ",
		renderBtn("e", "edit author"), " • ",
		renderBtn("d", "distribute times"), " • ",
		renderBtn("w", "apply/dry-run"), " • ",
		renderBtn("r", "rollback"), " • ",
		renderBtn("q", "quit"),
	)

	footer := lipgloss.JoinVertical(lipgloss.Left, " ", statusLine, helpBar)
	mainView := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	// 5. Render Modal Overlay if active
	if m.activeModal != ModalNone {
		var modalView string
		switch m.activeModal {
		case ModalEditAuthor:
			modalView = m.authorModal.View(m.width)
		case ModalTimeShift:
			modalView = m.timeModal.View(m.width)
		case ModalDryRun:
			modalView = m.dryRunModal.View(m.width, m.height)
		case ModalRollback:
			modalView = m.rollbackModal.View(m.width)
		}

		return placeOverlay(m.width, m.height, modalView)
	}

	return mainView
}

// placeOverlay renders modalView centered on screen
func placeOverlay(width, height int, modalView string) string {
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modalView,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func renderDetailRow(key, val string) string {
	return lipgloss.JoinHorizontal(lipgloss.Top, detailKeyStyle.Render(key), val)
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = -d
		if d < time.Minute {
			return fmt.Sprintf("in %d seconds", int(d.Seconds()))
		} else if d < time.Hour {
			return fmt.Sprintf("in %d minutes", int(d.Minutes()))
		}
		return fmt.Sprintf("in %d hours", int(d.Hours()))
	}
	if d < time.Minute {
		return fmt.Sprintf("%d seconds ago", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	}
	return fmt.Sprintf("%d hours ago", int(d.Hours()))
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running toolgit: %v\n", err)
		os.Exit(1)
	}
}
