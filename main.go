package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

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
		distributeSingleDayOrganic(commits, start, end, rng)
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

		daySlice := commits[commitIdx : commitIdx+numForDay]
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

type model struct {
	commits     []*CommitState
	cursor      int
	width       int
	height      int
	status      string
	statusOk    bool
	isRealRepo  bool
	currentBranch string

	// Modals
	activeModal ActiveModal
	authorModal EditAuthorModal
	timeModal   TimePickerModal
	dryRunModal DryRunModal
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

	statusMsg := "Ready. [e] Edit Author • [d] Distribute Times • [w] Apply/Dry-Run"
	if !isReal {
		statusMsg = "Mock Mode (No Git repo found). Changes will be simulated."
	}

	return model{
		commits:       commits,
		cursor:        0,
		status:        statusMsg,
		statusOk:      true,
		isRealRepo:    isReal,
		currentBranch: branch,
		activeModal:   ModalNone,
	}
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
	if len(m.commits) > 0 && m.cursor < len(m.commits) {
		return []*CommitState{m.commits[m.cursor]}, false
	}
	return nil, false
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			} else {
				m.cursor = len(m.commits) - 1
			}

		case "down", "j":
			if m.cursor < len(m.commits)-1 {
				m.cursor++
			} else {
				m.cursor = 0
			}

		case " ":
			if len(m.commits) > 0 {
				m.commits[m.cursor].Selected = !m.commits[m.cursor].Selected
				m.status = fmt.Sprintf("Toggled commit %s", m.commits[m.cursor].Hash[:7])
				m.statusOk = true
			}

		case "a":
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

		case "e":
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

		case "d":
			targets, _ := m.getTargetCommits()
			if len(targets) == 0 {
				m.status = "No commits available for time distribution."
				m.statusOk = false
				return m, nil
			}
			m.timeModal = NewTimePickerModal(len(targets))
			m.activeModal = ModalTimePicker

		case "t":
			// Quick distribute selected (09:00 - 17:00 today)
			targets, _ := m.getTargetCommits()
			if len(targets) == 0 {
				m.status = "No commits selected! Press [Space] to select commits."
				m.statusOk = false
				return m, nil
			}
			now := time.Now()
			start := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
			end := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
			DistributeTimes(targets, start, end)
			m.status = fmt.Sprintf("✓ Quick distributed %d commit(s) (09:00 - 17:00)", len(targets))
			m.statusOk = true

		case "w", "r":
			// Open Dry-Run / Rewrite Modal
			diffs := GenerateDryRunDiff(m.commits)
			m.dryRunModal = DryRunModal{
				Diffs:      diffs,
				IsRealRepo: m.isRealRepo,
			}
			m.activeModal = ModalDryRun
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

	case ModalTimePicker:
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

	case ModalDryRun:
		switch msg.String() {
		case "esc", "n", "N":
			m.activeModal = ModalNone
			return m, nil

		case "enter", "y", "Y":
			if m.isRealRepo {
				backup, err := ExecuteHistoryRewrite(m.commits)
				if err != nil {
					m.dryRunModal.ErrorMsg = err.Error()
					return m, nil
				}
				m.activeModal = ModalNone
				m.status = fmt.Sprintf("🚀 Rewrote history! Backup branch: %s", backup)
				m.statusOk = true
			} else {
				// Mock mode in-memory sync
				for _, c := range m.commits {
					c.OriginalName = c.AuthorName
					c.OriginalMail = c.AuthorEmail
					c.OriginalTime = c.Timestamp
					c.Selected = false
				}
				m.activeModal = ModalNone
				m.status = "✓ [Mock Mode] Applied changes in memory successfully."
				m.statusOk = true
			}
			return m, nil
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

	// Layout dimensions
	headerHeight := 2
	footerHeight := 3
	paneHeight := m.height - headerHeight - footerHeight
	if paneHeight < 8 {
		paneHeight = 8
	}

	// Balanced proportional split: Commit list gets ~48% width, details get ~52%
	leftWidth := (m.width * 48) / 100
	if leftWidth < 38 {
		leftWidth = 38
	}
	rightWidth := m.width - leftWidth - 3
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

	// 2. Left Pane (Commit List with windowed scrolling)
	// Calculate visible row capacity
	visibleRows := paneHeight - 3
	if visibleRows < 1 {
		visibleRows = 1
	}

	totalCommits := len(m.commits)
	startIdx := 0
	if m.cursor >= visibleRows {
		startIdx = m.cursor - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > totalCommits {
		endIdx = totalCommits
		if endIdx-visibleRows >= 0 {
			startIdx = endIdx - visibleRows
		} else {
			startIdx = 0
		}
	}

	// Calculate inner item width to guarantee single-line rendering without wrapping
	itemWidth := leftWidth - 4
	if itemWidth < 20 {
		itemWidth = 20
	}
	// Row item padding is 0, 1 -> available text width is itemWidth - 2
	contentWidth := itemWidth - 2

	var listItems []string
	for i := startIdx; i < endIdx && i < totalCommits; i++ {
		c := m.commits[i]
		shortHash := c.Hash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		check := "[ ]"
		if c.Selected {
			check = "[✓]"
		}

		modMarker := " "
		if c.AuthorName != c.OriginalName || c.AuthorEmail != c.OriginalMail || !c.Timestamp.Equal(c.OriginalTime) {
			modMarker = "✎"
		}

		// Fixed prefix: " ✎ [✓] 7f3a9b2 " ~ 14 characters
		prefixLen := 14
		availMsg := contentWidth - prefixLen
		if availMsg < 6 {
			availMsg = 6
		}

		msg := c.Message
		runes := []rune(msg)
		if len(runes) > availMsg {
			if availMsg > 3 {
				msg = string(runes[:availMsg-1]) + "…"
			} else {
				msg = string(runes[:availMsg])
			}
		}

		rowText := fmt.Sprintf("%-2s%-4s%-8s%s", modMarker, check, shortHash, msg)

		if i == m.cursor {
			listItems = append(listItems, cursorItemStyle.Width(itemWidth).Render(rowText))
		} else {
			listItems = append(listItems, normalItemStyle.Width(itemWidth).Render(rowText))
		}
	}

	// Scroll position indicator
	scrollInfo := fmt.Sprintf("%d/%d", m.cursor+1, totalCommits)
	if totalCommits == 0 {
		scrollInfo = "0/0"
	}

	headerSpacing := itemWidth - lipgloss.Width("Commits") - lipgloss.Width(scrollInfo)
	if headerSpacing < 1 {
		headerSpacing = 1
	}
	listHeader := lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Bold(true).Foreground(highlightColor).Render("Commits"),
		strings.Repeat(" ", headerSpacing),
		lipgloss.NewStyle().Foreground(subtleColor).Render(scrollInfo),
	)

	listContent := lipgloss.JoinVertical(lipgloss.Left, listItems...)
	leftPane := activePaneBorder.
		Width(leftWidth).
		Height(paneHeight).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			listHeader,
			"",
			listContent,
		))

	// 3. Right Pane (Commit Details)
	var rightPaneContent string
	if len(m.commits) > 0 && m.cursor < len(m.commits) {
		cur := m.commits[m.cursor]

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
		fmt.Sprintf("[%d selected / %d total]", selectedCount, len(m.commits)),
	)

	statusLine := fmt.Sprintf("%s %s  %s", statusIcon, m.status, countsInfo)

	renderKey := func(key, desc string) string {
		return lipgloss.JoinHorizontal(
			lipgloss.Center,
			keyBadgeStyle.Render(key),
			" ",
			keyDescStyle.Render(desc),
		)
	}

	helpBar := lipgloss.JoinHorizontal(
		lipgloss.Center,
		renderKey("↑/↓", "Nav"), "  ",
		renderKey("Space", "Select"), "  ",
		renderKey("a", "All"), "  ",
		renderKey("e", "Author"), "  ",
		renderKey("d", "Times"), "  ",
		renderKey("w", "Rewrite"), "  ",
		renderKey("q", "Quit"),
	)

	footer := lipgloss.JoinVertical(lipgloss.Left, "", statusLine, helpBar)
	mainView := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)

	// 5. Render Modal Overlay if active
	if m.activeModal != ModalNone {
		var modalView string
		switch m.activeModal {
		case ModalEditAuthor:
			modalView = m.authorModal.View(m.width)
		case ModalTimePicker:
			modalView = m.timeModal.View(m.width)
		case ModalDryRun:
			modalView = m.dryRunModal.View(m.width, m.height)
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
