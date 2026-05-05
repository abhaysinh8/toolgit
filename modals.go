package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// ActiveModal enum represents the current modal dialog state.
type ActiveModal int

const (
	ModalNone ActiveModal = iota
	ModalEditAuthor
	ModalTimePicker
	ModalDryRun
)

// --- Author / Email Modal ---

type EditAuthorModal struct {
	NameInput  textinput.Model
	EmailInput textinput.Model
	focusIndex int
	targetCount int
	isBatch     bool
}

func NewEditAuthorModal(defaultName, defaultEmail string, count int, isBatch bool) EditAuthorModal {
	ni := textinput.New()
	ni.Placeholder = "Author Name (e.g., Alex Chen)"
	ni.SetValue(defaultName)
	ni.Focus()
	ni.Prompt = "👤 Name:  "
	ni.CharLimit = 100
	ni.Width = 36

	ei := textinput.New()
	ei.Placeholder = "Author Email (e.g., alex.chen@example.com)"
	ei.SetValue(defaultEmail)
	ei.Prompt = "✉️  Email: "
	ei.CharLimit = 100
	ei.Width = 36

	return EditAuthorModal{
		NameInput:   ni,
		EmailInput:  ei,
		focusIndex:  0,
		targetCount: count,
		isBatch:     isBatch,
	}
}

func (m *EditAuthorModal) NextField() {
	m.focusIndex = (m.focusIndex + 1) % 2
	if m.focusIndex == 0 {
		m.NameInput.Focus()
		m.EmailInput.Blur()
	} else {
		m.NameInput.Blur()
		m.EmailInput.Focus()
	}
}

func (m *EditAuthorModal) PrevField() {
	m.focusIndex = (m.focusIndex + 1) % 2
	if m.focusIndex == 0 {
		m.NameInput.Focus()
		m.EmailInput.Blur()
	} else {
		m.NameInput.Blur()
		m.EmailInput.Focus()
	}
}

func (m EditAuthorModal) View(maxWidth int) string {
	scopeText := fmt.Sprintf("Applying to: %d selected commit(s)", m.targetCount)
	if !m.isBatch {
		scopeText = "Applying to: 1 active commit"
	}

	scopeBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render(scopeText)

	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		MarginBottom(1).
		Render("✏️  Edit Author & Email")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		scopeBadge,
		"",
		m.NameInput.View(),
		"",
		m.EmailInput.View(),
		"",
		lipgloss.NewStyle().Foreground(subtleColor).Render("[Tab/Shift+Tab] Next/Prev  •  [Enter] Apply  •  [Esc] Cancel"),
	)

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2).
		Width(50).
		Render(content)

	return modalBox
}

// --- Time Distribution Modal with Preset Dropdown & Custom Range ---

type TimePreset struct {
	Label       string
	Description string
	GetRange    func() (time.Time, time.Time)
}

type TimePickerModal struct {
	Presets        []TimePreset
	selectedPreset int // index in Presets
	isCustomMode   bool
	customFocus    int // 0 = Start, 1 = End, 2 = Days
	StartInput     textinput.Model
	EndInput       textinput.Model
	DaysInput      textinput.Model
	targetCount    int
	errMsg         string
}

func getPresets() []TimePreset {
	return []TimePreset{
		{
			Label:       "Today Workday",
			Description: "09:00 AM – 05:00 PM today (Organic jitter)",
			GetRange: func() (time.Time, time.Time) {
				now := time.Now()
				s := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
				e := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
				return s, e
			},
		},
		{
			Label:       "Yesterday Workday",
			Description: "09:00 AM – 05:00 PM yesterday (Organic jitter)",
			GetRange: func() (time.Time, time.Time) {
				yest := time.Now().AddDate(0, 0, -1)
				s := time.Date(yest.Year(), yest.Month(), yest.Day(), 9, 0, 0, 0, yest.Location())
				e := time.Date(yest.Year(), yest.Month(), yest.Day(), 17, 0, 0, 0, yest.Location())
				return s, e
			},
		},
		{
			Label:       "Past 3 Hours",
			Description: "3 hours ago until now (Organic jitter)",
			GetRange: func() (time.Time, time.Time) {
				now := time.Now()
				return now.Add(-3 * time.Hour), now
			},
		},
		{
			Label:       "Past 8 Hours",
			Description: "8 hours ago until now (Organic jitter)",
			GetRange: func() (time.Time, time.Time) {
				now := time.Now()
				return now.Add(-8 * time.Hour), now
			},
		},
		{
			Label:       "Custom Range...",
			Description: "Custom date range & active days spread",
			GetRange: func() (time.Time, time.Time) {
				now := time.Now()
				s := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
				e := time.Date(now.Year(), now.Month(), now.Day(), 17, 0, 0, 0, now.Location())
				return s, e
			},
		},
	}
}

func NewTimePickerModal(count int) TimePickerModal {
	presets := getPresets()
	s, e := presets[0].GetRange()

	si := textinput.New()
	si.Placeholder = "YYYY-MM-DD HH:MM"
	si.SetValue(s.Format("2006-01-02 15:04"))
	si.Prompt = "🕒 Start:        "
	si.Width = 22

	ei := textinput.New()
	ei.Placeholder = "YYYY-MM-DD HH:MM"
	ei.SetValue(e.Format("2006-01-02 15:04"))
	ei.Prompt = "🕒 End:          "
	ei.Width = 22

	di := textinput.New()
	di.Placeholder = "Auto (e.g. 10)"
	di.Prompt = "📅 Active Days:  "
	di.Width = 22

	return TimePickerModal{
		Presets:        presets,
		selectedPreset: 0,
		isCustomMode:   false,
		customFocus:    0,
		StartInput:     si,
		EndInput:       ei,
		DaysInput:      di,
		targetCount:    count,
	}
}

func (m *TimePickerModal) NextPreset() {
	if m.isCustomMode {
		return
	}
	if m.selectedPreset < len(m.Presets)-1 {
		m.selectedPreset++
		m.applyPresetPreview()
	}
}

func (m *TimePickerModal) PrevPreset() {
	if m.isCustomMode {
		return
	}
	if m.selectedPreset > 0 {
		m.selectedPreset--
		m.applyPresetPreview()
	}
}

func (m *TimePickerModal) applyPresetPreview() {
	m.errMsg = ""
	if m.selectedPreset < len(m.Presets)-1 {
		s, e := m.Presets[m.selectedPreset].GetRange()
		m.StartInput.SetValue(s.Format("2006-01-02 15:04"))
		m.EndInput.SetValue(e.Format("2006-01-02 15:04"))
		m.DaysInput.SetValue("")
	}
}

func (m *TimePickerModal) ActivateCustomMode() {
	m.isCustomMode = true
	m.customFocus = 0
	m.StartInput.Focus()
	m.EndInput.Blur()
	m.DaysInput.Blur()
}

func (m *TimePickerModal) NextCustomField() {
	if !m.isCustomMode {
		return
	}
	m.customFocus = (m.customFocus + 1) % 3
	switch m.customFocus {
	case 0:
		m.StartInput.Focus()
		m.EndInput.Blur()
		m.DaysInput.Blur()
	case 1:
		m.StartInput.Blur()
		m.EndInput.Focus()
		m.DaysInput.Blur()
	case 2:
		m.StartInput.Blur()
		m.EndInput.Blur()
		m.DaysInput.Focus()
	}
}

func (m *TimePickerModal) PrevCustomField() {
	if !m.isCustomMode {
		return
	}
	m.customFocus = (m.customFocus + 2) % 3
	switch m.customFocus {
	case 0:
		m.StartInput.Focus()
		m.EndInput.Blur()
		m.DaysInput.Blur()
	case 1:
		m.StartInput.Blur()
		m.EndInput.Focus()
		m.DaysInput.Blur()
	case 2:
		m.StartInput.Blur()
		m.EndInput.Blur()
		m.DaysInput.Focus()
	}
}

func (m *TimePickerModal) SwitchToPresets() {
	m.isCustomMode = false
	m.StartInput.Blur()
	m.EndInput.Blur()
	m.DaysInput.Blur()
	if m.selectedPreset == len(m.Presets)-1 {
		m.selectedPreset = 0
		m.applyPresetPreview()
	}
}

func (m TimePickerModal) ParseTimes() (time.Time, time.Time, int, error) {
	const layout = "2006-01-02 15:04"
	st, err := time.ParseInLocation(layout, strings.TrimSpace(m.StartInput.Value()), time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, 0, fmt.Errorf("invalid Start Time (use YYYY-MM-DD HH:MM)")
	}
	et, err := time.ParseInLocation(layout, strings.TrimSpace(m.EndInput.Value()), time.Local)
	if err != nil {
		return time.Time{}, time.Time{}, 0, fmt.Errorf("invalid End Time (use YYYY-MM-DD HH:MM)")
	}
	if !et.After(st) {
		return time.Time{}, time.Time{}, 0, fmt.Errorf("End Time must be after Start Time")
	}

	minDays := 0
	daysStr := strings.TrimSpace(m.DaysInput.Value())
	if daysStr != "" {
		d, err := strconv.Atoi(daysStr)
		if err != nil || d < 1 {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("Active Days must be a positive number")
		}
		sDay := time.Date(st.Year(), st.Month(), st.Day(), 0, 0, 0, 0, st.Location())
		eDay := time.Date(et.Year(), et.Month(), et.Day(), 0, 0, 0, 0, et.Location())
		calendarDays := int(eDay.Sub(sDay).Hours()/24) + 1
		if d > calendarDays {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("Active Days (%d) cannot exceed total span (%d days)", d, calendarDays)
		}
		minDays = d
	}

	return st, et, minDays, nil
}

func (m TimePickerModal) View(maxWidth int) string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		MarginBottom(1).
		Render("⏳  Select Time Distribution Window")

	scopeBadge := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render(fmt.Sprintf("Distributing across %d selected commit(s)", m.targetCount))

	// Render Preset Options (Radio / Dropdown list)
	var presetRows []string
	presetRows = append(presetRows, lipgloss.NewStyle().Bold(true).Foreground(subtleColor).Render("Select Preset Range:"))

	for i, p := range m.Presets {
		isHighlighted := (!m.isCustomMode && i == m.selectedPreset) || (m.isCustomMode && i == len(m.Presets)-1)

		radio := "( )"
		if i == m.selectedPreset {
			radio = "(●)"
		}

		labelStr := fmt.Sprintf("%s %-18s %s", radio, p.Label, lipgloss.NewStyle().Foreground(subtleColor).Render(p.Description))

		if isHighlighted {
			presetRows = append(presetRows, lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#5A3EBC")).
				Padding(0, 1).
				Render("▸ "+labelStr))
		} else {
			presetRows = append(presetRows, lipgloss.NewStyle().
				Foreground(textColor).
				Padding(0, 1).
				Render("  "+labelStr))
		}
	}

	presetList := lipgloss.JoinVertical(lipgloss.Left, presetRows...)

	// Custom fields view (visible and active if custom mode selected)
	var customInputsView string
	if m.isCustomMode {
		customInputsView = lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			lipgloss.NewStyle().Bold(true).Foreground(highlightColor).Render("Custom Range & Active Days:"),
			m.StartInput.View(),
			m.EndInput.View(),
			m.DaysInput.View(),
		)
	}

	// Calculate and display preview
	var previewText string
	st, et, minDays, err := m.ParseTimes()
	if err == nil {
		sDay := time.Date(st.Year(), st.Month(), st.Day(), 0, 0, 0, 0, st.Location())
		eDay := time.Date(et.Year(), et.Month(), et.Day(), 0, 0, 0, 0, et.Location())
		calendarDays := int(eDay.Sub(sDay).Hours()/24) + 1

		var summary string
		if calendarDays > 1 {
			effectiveDays := minDays
			if effectiveDays <= 0 {
				effectiveDays = int(math.Ceil(float64(m.targetCount) / 2.5))
				if effectiveDays > calendarDays {
					effectiveDays = calendarDays
				}
				if effectiveDays > m.targetCount {
					effectiveDays = m.targetCount
				}
				if effectiveDays < 1 {
					effectiveDays = 1
				}
			}
			summary = fmt.Sprintf("Span: %d days  •  Active Days: ~%d  •  🌿 Organic Jitter", calendarDays, effectiveDays)
		} else {
			summary = "Single Day  •  🌿 Organic Human Jitter (Natural Gaps)"
		}

		previewText = lipgloss.NewStyle().Foreground(accentColor).Render(
			fmt.Sprintf("Window: %s → %s\n%s", st.Format("Jan 02, 15:04"), et.Format("Jan 02, 15:04"), summary),
		)
	}

	var errView string
	if m.errMsg != "" {
		errView = lipgloss.NewStyle().Foreground(warnColor).Bold(true).Render("⚠️  " + m.errMsg)
	}

	helpText := "[↑/↓/j/k] Select Preset  •  [Enter] Apply  •  [Esc] Cancel"
	if m.isCustomMode {
		helpText = "[Tab] Next Field  •  [Esc] Back to Presets  •  [Enter] Apply"
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		scopeBadge,
		"",
		presetList,
		customInputsView,
		"",
		previewText,
		errView,
		"",
		lipgloss.NewStyle().Foreground(subtleColor).Render(helpText),
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2).
		Width(64).
		Render(content)
}

// --- Dry-Run & Rebase Confirmation Modal ---

type DryRunModal struct {
	Diffs        []DiffItem
	BackupBranch string
	IsRealRepo   bool
	ErrorMsg     string
}

func (m DryRunModal) View(maxWidth, maxHeight int) string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render("🔍 Dry-Run: Review Git History Rewrite")

	repoNotice := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render(
		"✓ Real Git Repository detected. Automatic safety backup branch will be created.",
	)
	if !m.IsRealRepo {
		repoNotice = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5C07B")).Bold(true).Render(
			"ℹ️ Mock Mode: Applying changes locally in memory (No git repo present).",
		)
	}

	// Count modified
	modCount := 0
	for _, d := range m.Diffs {
		if d.IsModified {
			modCount++
		}
	}

	summaryLine := fmt.Sprintf("Commits to rewrite: %d total (%d modified)", len(m.Diffs), modCount)

	// Build diff table
	var rows []string
	tableHeader := fmt.Sprintf("%-8s | %-18s | %-24s | %-24s", "HASH", "AUTHOR", "OLD DATE", "NEW DATE")
	rows = append(rows, lipgloss.NewStyle().Bold(true).Foreground(subtleColor).Render(tableHeader))
	rows = append(rows, lipgloss.NewStyle().Foreground(subtleColor).Render(strings.Repeat("─", 80)))

	for _, d := range m.Diffs {
		shortHash := d.OldHash
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}

		authorDisp := d.NewAuthor
		if len(authorDisp) > 17 {
			authorDisp = authorDisp[:14] + "..."
		}

		oldDate := d.OldTime.Format("01/02 15:04:05")
		newDate := d.NewTime.Format("01/02 15:04:05")

		statusMarker := " "
		rowStyle := lipgloss.NewStyle().Foreground(textColor)
		if d.IsModified {
			statusMarker = "✎"
			rowStyle = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
		}

		rowStr := fmt.Sprintf("%s%-7s | %-18s | %-24s | %-24s", statusMarker, shortHash, authorDisp, oldDate, newDate)
		rows = append(rows, rowStyle.Render(rowStr))
	}

	tableView := lipgloss.JoinVertical(lipgloss.Left, rows...)

	actions := lipgloss.NewStyle().Bold(true).Render(
		"[Enter / y] Confirm & Rewrite History   •   [Esc / n] Cancel",
	)

	var errDisplay string
	if m.ErrorMsg != "" {
		errDisplay = lipgloss.NewStyle().Foreground(warnColor).Bold(true).Render("❌ " + m.ErrorMsg)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		repoNotice,
		summaryLine,
		"",
		tableView,
		"",
		errDisplay,
		actions,
	)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(highlightColor).
		Padding(1, 2).
		Width(86).
		Render(content)
}
