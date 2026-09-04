package app

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"

	"toolgit/internal/core"
	"toolgit/internal/git"
)

type RewriteFinishedMsg struct {
	BackupBranch string
	Err          error
}

type RollbackFinishedMsg struct {
	Branch string
	Err    error
}

type BranchSwitchFinishedMsg struct {
	Branch string
	Err    error
}

// DistributeTimes spaces commits naturally with realistic human jitter across the time window.
func DistributeTimes(commits []*core.CommitState, start time.Time, end time.Time) {
	DistributeTimesOrganic(commits, start, end, 0)
}

func calendarDaySpan(start, end time.Time) int {
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	days := 1
	for day := startDay; day.Before(endDay); day = day.AddDate(0, 0, 1) {
		days++
	}
	return days
}

// DistributeTimesOrganic distributes commits with organic human jitter, natural seconds, and active days partitioning.
func DistributeTimesOrganic(commits []*core.CommitState, start time.Time, end time.Time, minDays int) {
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
	chronoCommits := make([]*core.CommitState, n)
	for i := 0; i < n; i++ {
		chronoCommits[i] = commits[n-1-i]
	}

	// Calculate calendar span
	sDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	calendarDays := calendarDaySpan(start, end)

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
		if dayEnd.Before(dayStart) {
			// The requested range does not intersect normal working hours on
			// this day (for example, a 23:00→01:00 overnight range). Fall
			// back to the part of the requested range that lies on this day.
			dayStart = curDate
			if start.After(dayStart) {
				dayStart = start
			}
			dayEnd = curDate.AddDate(0, 0, 1).Add(-time.Nanosecond)
			if end.Before(dayEnd) {
				dayEnd = end
			}
		}

		daySlice := chronoCommits[commitIdx : commitIdx+numForDay]
		distributeSingleDayOrganic(daySlice, dayStart, dayEnd, rng)
		commitIdx += numForDay
	}
}

func distributeSingleDayOrganic(commits []*core.CommitState, start, end time.Time, rng *rand.Rand) {
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
func generateMockCommits() []*core.CommitState {
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

	var list []*core.CommitState
	for _, m := range mockData {
		t := now.Add(m.offset)
		list = append(list, &core.CommitState{
			Hash:                  m.hash,
			OriginalHash:          m.hash,
			Message:               m.msg,
			AuthorName:            m.name,
			OriginalName:          m.name,
			AuthorEmail:           m.email,
			OriginalMail:          m.email,
			Timestamp:             t,
			OriginalTime:          t,
			CommitterName:         m.name,
			OriginalCommitterName: m.name,
			CommitterEmail:        m.email,
			OriginalCommitterMail: m.email,
			CommitterTime:         t,
			OriginalCommitterTime: t,
			Selected:              false,
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
	Branch    key.Binding
	Quit      key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Select, k.Edit, k.Time, k.Apply, k.Rollback, k.Branch, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.SelectAll},
		{k.Edit, k.Time, k.Apply, k.Rollback, k.Branch, k.Quit},
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
	Branch:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "branch")),
	Quit:      key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q", "quit")),
}

type model struct {
	commits       []*core.CommitState
	table         table.Model
	help          help.Model
	keys          keyMap
	width         int
	height        int
	status        string
	statusOk      bool
	isRealRepo    bool
	currentBranch string

	// Modals
	activeModal   ActiveModal
	authorModal   EditAuthorModal
	timeModal     TimePickerModal
	dryRunModal   DryRunModal
	rollbackModal RollbackModal
	branchModal   BranchSwitchModal
}

func initialModel() model {
	var commits []*core.CommitState
	isReal := false
	branch := ""
	var loadErr error

	if git.IsInsideGitRepo() {
		isReal = true
		branch, _ = git.GetCurrentBranch()
		realCommits, err := git.LoadGitCommits()
		if err == nil {
			commits = realCommits
		} else {
			loadErr = err
		}
	}

	if !isReal {
		commits = generateMockCommits()
	}

	statusMsg := "Ready."
	if !isReal {
		statusMsg = "Mock Mode (No Git repo found). Changes will be simulated."
	} else if errors.Is(loadErr, git.ErrNoUnpushedCommits) {
		statusMsg = "No unpushed commits found on the current branch."
	} else if loadErr != nil {
		statusMsg = "Unable to load Git commits: " + loadErr.Error()
	}

	return newModel(commits, isReal, branch, statusMsg)
}

func newModel(commits []*core.CommitState, isReal bool, branch, statusMsg string) model {

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

	termW, termH, err := term.GetSize(os.Stdout.Fd())
	if err != nil || termW <= 0 || termH <= 0 {
		termW = 80
		termH = 24
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
		width:         termW,
		height:        termH,
	}
	m.resizeUI()
	return m
}

func (m *model) resizeUI() {
	if m.width < 40 {
		m.width = 40
	}
	if m.height < 10 {
		m.height = 10
	}

	paneInnerHeight := m.height - 6
	if paneInnerHeight < 4 {
		paneInnerHeight = 4
	}

	leftOuterWidth := (m.width * 44) / 100
	if leftOuterWidth < 28 {
		leftOuterWidth = 28
	}

	leftPrintableWidth := leftOuterWidth - 4
	if leftPrintableWidth < 24 {
		leftPrintableWidth = 24
	}

	tableDataHeight := paneInnerHeight - 2
	if tableDataHeight < 1 {
		tableDataHeight = 1
	}

	m.table.SetHeight(tableDataHeight)
	m.table.SetWidth(leftPrintableWidth)

	msgColWidth := leftPrintableWidth - 17
	if msgColWidth < 6 {
		msgColWidth = 6
	}

	m.table.SetColumns([]table.Column{
		{Title: " ", Width: 4},
		{Title: "Hash", Width: 7},
		{Title: "Message", Width: msgColWidth},
	})
	m.updateTable()
}

func (m *model) updateTable() {
	cols := m.table.Columns()
	maxMsgLen := 45
	if len(cols) >= 3 && cols[2].Width > 3 {
		maxMsgLen = cols[2].Width
	}

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
		mergeMarker := " "
		if c.IsMerge {
			mergeMarker = "M"
		}
		status := fmt.Sprintf("%s%s %s", modMarker, mergeMarker, check)

		msg := strings.SplitN(c.Message, "\n", 2)[0]
		msg = truncateForDisplay(msg, maxMsgLen)
		rows = append(rows, table.Row{status, shortHash, msg})
	}
	m.table.SetRows(rows)
}

func (m *model) reloadCommits() error {
	commits, err := git.LoadGitCommits()
	if err != nil && !errors.Is(err, git.ErrNoUnpushedCommits) {
		return err
	}
	m.commits = commits
	m.table.SetCursor(0)
	m.updateTable()
	return nil
}

func truncateForDisplay(value string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(value, width, "...")
}

func (m model) Init() tea.Cmd {
	return nil
}

// getTargetCommits returns selected commits, or if none selected, the currently hovered commit.
func (m model) getTargetCommits() ([]*core.CommitState, bool) {
	var selected []*core.CommitState
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
		return []*core.CommitState{m.commits[cursor]}, false
	}
	return nil, false
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeUI()
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
		} else if m.activeModal == ModalBranchSwitch && m.branchModal.State == BranchSwitchSwitching {
			var cmd tea.Cmd
			m.branchModal.Spinner, cmd = m.branchModal.Spinner.Update(msg)
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
				if err := m.reloadCommits(); err != nil {
					m.status = "Rollback succeeded, but commits could not be reloaded: " + err.Error()
					m.statusOk = false
				}
			}
			m.activeModal = ModalNone
		}
		return m, nil

	case BranchSwitchFinishedMsg:
		if m.activeModal == ModalBranchSwitch {
			if msg.Err != nil {
				m.branchModal.State = BranchSwitchIdle
				m.branchModal.ErrorMsg = msg.Err.Error()
				m.status = "Branch switch failed: " + msg.Err.Error()
				m.statusOk = false
			} else {
				m.currentBranch = msg.Branch
				m.status = fmt.Sprintf("Switched to branch '%s'", msg.Branch)
				m.statusOk = true
				if err := m.reloadCommits(); err != nil {
					m.status = "Branch switched, but commits could not be reloaded: " + err.Error()
					m.statusOk = false
				}
				m.activeModal = ModalNone
			}
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
			if len(m.commits) == 0 {
				m.status = "No commits are available to rewrite."
				m.statusOk = false
				return m, nil
			}
			// Open Dry-Run / Rewrite Modal
			diffs := git.GenerateDryRunDiff(m.commits)
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
			backups, err := git.GetBackupBranches()
			if err != nil || len(backups) == 0 {
				m.status = "No backup branches found for rollback."
				m.statusOk = false
				return m, nil
			}
			m.rollbackModal = NewRollbackModal(backups)
			m.activeModal = ModalRollback
		case key.Matches(msg, m.keys.Branch):
			if !m.isRealRepo {
				m.status = "Branch switching is not available in mock mode."
				m.statusOk = false
				return m, nil
			}
			branches, current, err := git.GetLocalBranches()
			if err != nil || len(branches) == 0 {
				m.status = "No local branches found."
				m.statusOk = false
				return m, nil
			}
			// Check if user has unsaved edits
			hasDirty := false
			for _, c := range m.commits {
				if c.AuthorName != c.OriginalName || c.AuthorEmail != c.OriginalMail || !c.Timestamp.Equal(c.OriginalTime) {
					hasDirty = true
					break
				}
			}
			m.branchModal = NewBranchSwitchModal(branches, current, hasDirty)
			m.activeModal = ModalBranchSwitch
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
		switch m.rollbackModal.State {
		case RollbackStateExecuting:
			return m, nil
		case RollbackStateConfirm:
			switch msg.String() {
			case "enter", "y", "Y":
				branch := m.rollbackModal.PendingBranch
				m.rollbackModal.State = RollbackStateExecuting
				return m, tea.Batch(
					m.rollbackModal.Spinner.Tick,
					func() tea.Msg {
						err := git.ExecuteRollback(branch)
						return RollbackFinishedMsg{Branch: branch, Err: err}
					},
				)
			case "esc", "n", "N":
				m.rollbackModal.State = RollbackStateSelect
				m.rollbackModal.PendingBranch = ""
				return m, nil
			}
		default:
			switch msg.String() {
			case "esc", "q":
				m.activeModal = ModalNone
				return m, nil
			case "enter":
				selected := m.rollbackModal.List.SelectedItem()
				if selected != nil {
					m.rollbackModal.PendingBranch = selected.FilterValue()
					m.rollbackModal.State = RollbackStateConfirm
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.rollbackModal.List, cmd = m.rollbackModal.List.Update(msg)
				return m, cmd
			}
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
							backup, err := git.ExecuteHistoryRewrite(commitsToRewrite)
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
					if err := m.reloadCommits(); err != nil {
						m.status = "Rewrite completed, but commits could not be reloaded: " + err.Error()
						m.statusOk = false
					}
				}
				return m, nil
			}
		}

	case ModalBranchSwitch:
		switch m.branchModal.State {
		case BranchSwitchSwitching:
			// Ignore all input while switching
			return m, nil
		case BranchSwitchConfirm:
			switch msg.String() {
			case "enter", "y", "Y":
				// Confirmed: proceed with switch
				branch := m.branchModal.PendingBranch
				m.branchModal.State = BranchSwitchSwitching
				return m, tea.Batch(
					m.branchModal.Spinner.Tick,
					func() tea.Msg {
						err := git.SwitchBranch(branch)
						return BranchSwitchFinishedMsg{Branch: branch, Err: err}
					},
				)
			case "esc", "n", "N":
				m.branchModal.State = BranchSwitchIdle
				m.branchModal.PendingBranch = ""
				return m, nil
			}
		default:
			// Idle state: list navigation
			switch msg.String() {
			case "esc", "q":
				m.activeModal = ModalNone
				return m, nil
			case "enter":
				selected := m.branchModal.List.SelectedItem()
				if selected != nil {
					item := selected.(switchBranchItem)
					if item.name == m.branchModal.CurrentBranch {
						// Already on this branch, just close
						m.activeModal = ModalNone
						return m, nil
					}
					if m.branchModal.HasDirtyEdits {
						// Show confirmation first
						m.branchModal.PendingBranch = item.name
						m.branchModal.State = BranchSwitchConfirm
						return m, nil
					}
					// No dirty edits, switch directly
					branch := item.name
					m.branchModal.State = BranchSwitchSwitching
					return m, tea.Batch(
						m.branchModal.Spinner.Tick,
						func() tea.Msg {
							err := git.SwitchBranch(branch)
							return BranchSwitchFinishedMsg{Branch: branch, Err: err}
						},
					)
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.branchModal.List, cmd = m.branchModal.List.Update(msg)
				return m, cmd
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

	// Layout dimensions:
	// Total screen height = m.height
	// Header = 1 line
	// Footer = 3 lines (" ", statusLine, helpBar)
	// Pane borders (top + bottom) = 2 lines
	// So inner content height = m.height - 1 - 3 - 2 = m.height - 6
	paneInnerHeight := m.height - 6
	if paneInnerHeight < 4 {
		paneInnerHeight = 4
	}

	leftOuterWidth := (m.width * 44) / 100
	if leftOuterWidth < 28 {
		leftOuterWidth = 28
	}
	rightOuterWidth := m.width - leftOuterWidth
	if rightOuterWidth < 28 {
		rightOuterWidth = 28
	}

	leftPrintableWidth := leftOuterWidth - 4
	if leftPrintableWidth < 24 {
		leftPrintableWidth = 24
	}
	rightPrintableWidth := rightOuterWidth - 4
	if rightPrintableWidth < 24 {
		rightPrintableWidth = 24
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

	headerLeft := titleBadge.Render(" ❖ toolgit ")
	if m.width >= 70 {
		headerLeft += " " + lipgloss.NewStyle().Foreground(subtleColor).Render("Safe Git Commit Metadata Editor")
	}

	badgeW := lipgloss.Width(repoBadge)
	leftW := lipgloss.Width(headerLeft)
	headerGap := m.width - leftW - badgeW
	if headerGap < 1 {
		headerGap = 1
	}
	header := lipgloss.JoinHorizontal(lipgloss.Top,
		headerLeft,
		strings.Repeat(" ", headerGap),
		repoBadge,
	)
	if lipgloss.Width(header) > m.width {
		header = lipgloss.NewStyle().MaxWidth(m.width).Render(header)
	}

	// 2. Left Pane (Commit List with table)
	tableDataHeight := paneInnerHeight - 2 // 1 line for header + 1 line for header border
	if tableDataHeight < 1 {
		tableDataHeight = 1
	}
	m.table.SetHeight(tableDataHeight)
	m.table.SetWidth(leftPrintableWidth)

	msgColWidth := leftPrintableWidth - 16
	if msgColWidth < 6 {
		msgColWidth = 6
	}
	m.table.SetColumns([]table.Column{
		{Title: " ", Width: 3},
		{Title: "Hash", Width: 7},
		{Title: "Message", Width: msgColWidth},
	})

	listContent := m.table.View()
	leftPane := activePaneBorder.
		Width(leftOuterWidth - 2).
		Height(paneInnerHeight).
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

		var mergeBadge string
		if cur.IsMerge {
			mergeBadge = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#8B5CF6")).
				Padding(0, 1).
				Render("⑂ MERGE")
		}

		shortHash := cur.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		detailTitle := detailHeaderStyle.Render("Commit " + shortHash)
		var badges string
		if mergeBadge != "" {
			badges = lipgloss.JoinHorizontal(lipgloss.Left, statusBadge, "  ", selBadge, "  ", mergeBadge)
		} else {
			badges = lipgloss.JoinHorizontal(lipgloss.Left, statusBadge, "  ", selBadge)
		}

		var headerBlock string
		if rightPrintableWidth >= lipgloss.Width(detailTitle)+lipgloss.Width(badges)+4 {
			headerBlock = lipgloss.JoinHorizontal(lipgloss.Center, detailTitle, "  ", badges)
		} else {
			headerBlock = lipgloss.JoinVertical(lipgloss.Left, detailTitle, badges)
		}

		maxValW := rightPrintableWidth - 14
		if maxValW < 10 {
			maxValW = 10
		}

		shaDisplay := cur.Hash
		if maxValW < 40 {
			if maxValW >= 8 {
				shaDisplay = cur.Hash[:maxValW]
			}
		}

		details := []string{
			headerBlock,
			"",
			renderDetailRow("SHA:", hashStyle.Render(shaDisplay), maxValW),
			renderDetailRow("Author:", detailValStyle.Render(cur.AuthorName), maxValW),
			renderDetailRow("Author Email:", detailValStyle.Render(cur.AuthorEmail), maxValW),
			renderDetailRow("Author Date:", detailValStyle.Render(cur.Timestamp.Format("Mon Jan 02, 2006 • 15:04")), maxValW),
			renderDetailRow("Committer:", detailValStyle.Render(cur.CommitterName), maxValW),
			renderDetailRow("Commit Date:", detailValStyle.Render(cur.CommitterTime.Format("Mon Jan 02, 2006 • 15:04")), maxValW),
			renderDetailRow("Relative:", detailValStyle.Render(formatRelativeTime(cur.Timestamp)), maxValW),
		}

		if cur.IsMerge && len(cur.ParentHashes) > 1 {
			var parentStrs []string
			for _, p := range cur.ParentHashes {
				if len(p) > 7 {
					parentStrs = append(parentStrs, p[:7])
				} else {
					parentStrs = append(parentStrs, p)
				}
			}
			details = append(details, renderDetailRow("Parents:", detailValStyle.Render(strings.Join(parentStrs, ", ")), maxValW))
		}

		details = append(details, "", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A1A1AA")).Render("Message:"))

		msgBoxWidth := rightPrintableWidth - 2
		if msgBoxWidth < 10 {
			msgBoxWidth = 10
		}
		msgBoxHeight := paneInnerHeight - len(details) - 1
		if msgBoxHeight < 2 {
			msgBoxHeight = 2
		}

		msgBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(highlightColor).
			Padding(0, 1).
			Width(msgBoxWidth).
			MaxWidth(msgBoxWidth).
			Height(msgBoxHeight).
			MaxHeight(msgBoxHeight).
			Foreground(textColor).
			Render(cur.Message)

		details = append(details, msgBox)
		rightPaneContent = lipgloss.JoinVertical(lipgloss.Left, details...)
	} else {
		rightPaneContent = detailValStyle.Render("No commit selected.")
	}

	rightPane := inactivePaneBorder.
		Width(rightOuterWidth - 2).
		Height(paneInnerHeight).
		Render(rightPaneContent)

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	if lipgloss.Width(body) > m.width {
		body = lipgloss.NewStyle().MaxWidth(m.width).Render(body)
	}

	// 4. Footer: Status Line + Responsive Help Bar
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
	cursorNum := 0
	if len(m.commits) > 0 {
		cursorNum = m.table.Cursor() + 1
	}
	countsInfo := lipgloss.NewStyle().Foreground(subtleColor).Render(
		fmt.Sprintf("[%d/%d]  [%d selected]", cursorNum, len(m.commits), selectedCount),
	)

	statusLine := fmt.Sprintf("%s %s  %s", statusIcon, m.status, countsInfo)
	if lipgloss.Width(statusLine) > m.width {
		statusLine = lipgloss.NewStyle().MaxWidth(m.width).Render(statusLine)
	}

	// Responsive Help Bar
	helpKeyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F38BA8")).Bold(true)
	helpDescStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))

	renderBtn := func(key, desc string) string {
		return fmt.Sprintf("%s %s", helpKeyStyle.Render(key), helpDescStyle.Render(desc))
	}

	var helpBar string
	if m.width >= 105 {
		helpBar = lipgloss.JoinHorizontal(lipgloss.Left,
			renderBtn("space", "select"), " • ",
			renderBtn("a", "select all"), " • ",
			renderBtn("e", "edit author"), " • ",
			renderBtn("d", "distribute times"), " • ",
			renderBtn("w", "apply/dry-run"), " • ",
			renderBtn("r", "rollback"), " • ",
			renderBtn("b", "branch"), " • ",
			renderBtn("q", "quit"),
		)
	} else if m.width >= 75 {
		helpBar = lipgloss.JoinHorizontal(lipgloss.Left,
			renderBtn("space", "sel"), " • ",
			renderBtn("a", "all"), " • ",
			renderBtn("e", "edit"), " • ",
			renderBtn("d", "times"), " • ",
			renderBtn("w", "apply"), " • ",
			renderBtn("r", "rollback"), " • ",
			renderBtn("b", "branch"), " • ",
			renderBtn("q", "quit"),
		)
	} else {
		helpBar = lipgloss.JoinHorizontal(lipgloss.Left,
			renderBtn("space", "sel"), " • ",
			renderBtn("e", "edit"), " • ",
			renderBtn("d", "times"), " • ",
			renderBtn("w", "apply"), " • ",
			renderBtn("b", "branch"), " • ",
			renderBtn("q", "quit"),
		)
	}

	footer := lipgloss.JoinVertical(lipgloss.Left, " ", statusLine, helpBar)
	mainView := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	// Ensure view strictly fits within m.height and m.width
	lines := strings.Split(mainView, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i := range lines {
		if lipgloss.Width(lines[i]) > m.width {
			lines[i] = lipgloss.NewStyle().MaxWidth(m.width).Render(lines[i])
		}
	}
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	mainView = strings.Join(lines, "\n")

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
		case ModalBranchSwitch:
			modalView = m.branchModal.View(m.width, m.height)
		}

		return placeOverlay(m.width, m.height, modalView)
	}

	return mainView
}

// placeOverlay renders modalView centered on screen, strictly clamped to width and height
func placeOverlay(width, height int, modalView string) string {
	res := lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modalView,
		lipgloss.WithWhitespaceChars(" "),
	)
	lines := strings.Split(res, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i := range lines {
		if lipgloss.Width(lines[i]) > width {
			lines[i] = lipgloss.NewStyle().MaxWidth(width).Render(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

func renderDetailRow(key, val string, maxValWidth int) string {
	k := detailKeyStyle.Render(key)
	if maxValWidth > 0 && lipgloss.Width(val) > maxValWidth {
		val = lipgloss.NewStyle().MaxWidth(maxValWidth).Render(val)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, k, val)
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

type branchPickerItem struct {
	name      string
	isCurrent bool
}

func (i branchPickerItem) Title() string {
	if i.isCurrent {
		return i.name + " (current)"
	}
	return i.name
}

func (i branchPickerItem) Description() string {
	if i.isCurrent {
		return "currently checked out branch"
	}
	return "local branch"
}

func (i branchPickerItem) FilterValue() string { return i.name }

type branchPickerModel struct {
	list      list.Model
	selected  string
	cancelled bool
	quitting  bool
	width     int
	height    int
}

func newBranchPickerModel(branches []string, current string) branchPickerModel {
	items := make([]list.Item, len(branches))
	for i, b := range branches {
		items[i] = branchPickerItem{name: b, isCurrent: b == current}
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#6366F1")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#E0E7FF")).
		Background(lipgloss.Color("#6366F1"))

	l := list.New(items, delegate, 64, 16)
	l.Title = "toolgit • Select Branch"
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#6366F1")).
		Padding(0, 1)

	return branchPickerModel{
		list:     l,
		selected: current,
	}
}

func (m branchPickerModel) Init() tea.Cmd {
	return nil
}

func (m branchPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if sel := m.list.SelectedItem(); sel != nil {
				item := sel.(branchPickerItem)
				m.selected = item.name
			}
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m branchPickerModel) View() string {
	if m.quitting {
		return ""
	}
	content := m.list.View()
	framed := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2).
		Render(content)

	w := m.width
	h := m.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, framed)
}

func runBranchPicker(branches []string, current string) (string, bool) {
	picker := newBranchPickerModel(branches, current)
	p := tea.NewProgram(picker, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return current, false
	}
	m := finalModel.(branchPickerModel)
	if m.cancelled {
		return "", true
	}
	return m.selected, false
}

func runMainTUI() error {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func Start() error {
	return StartWithBranch("")
}

func StartWithBranch(targetBranch string) error {
	if targetBranch != "" {
		if git.IsInsideGitRepo() {
			if err := git.SwitchBranch(targetBranch); err != nil {
				return fmt.Errorf("failed to switch to branch '%s': %w", targetBranch, err)
			}
		}
		return runMainTUI()
	}

	if !git.IsInsideGitRepo() {
		return runMainTUI()
	}

	branches, current, err := git.GetLocalBranches()
	if err != nil || len(branches) <= 1 {
		return runMainTUI()
	}

	selected, cancelled := runBranchPicker(branches, current)
	if cancelled {
		return nil
	}

	if selected != "" && selected != current {
		if err := git.SwitchBranch(selected); err != nil {
			return fmt.Errorf("failed to switch branch to '%s': %w", selected, err)
		}
	}

	return runMainTUI()
}
