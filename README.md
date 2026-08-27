# 🛠️ toolgit

<p align="center">
  <strong>Fast, non-destructive Terminal User Interface (TUI) for safely inspecting, modifying, and batch-distributing Git commit metadata.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%201.21+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/TUI-Bubble%20Tea-7D56F4?style=flat-square" alt="Bubble Tea" />
  <img src="https://img.shields.io/badge/Styling-Lip%20Gloss-04B575?style=flat-square" alt="Lip Gloss" />
  <img src="https://img.shields.io/badge/Safety-Automated%20Backups-F59E0B?style=flat-square" alt="Safety Backups" />
</p>

---

## 📖 Table of Contents
- [✨ Key Features](#-key-features)
- [⌨️ Keyboard Shortcuts](#️-keyboard-shortcuts)
- [🚀 Quick Start & Installation](#-quick-start--installation)
- [🌿 Organic Time Jitter & Active Days](#-organic-time-jitter--active-days)
- [🛡️ Safety & Git Plumbing Engine](#️-safety--git-plumbing-engine)
- [🧪 Running the Test Suite](#-running-the-test-suite)
- [📂 Project Architecture](#-project-architecture)
- [📚 Deep-Dive Documentation](#-deep-dive-documentation)

---

## ✨ Key Features

| Feature | Description |
|---|---|
| 🖥️ **Split-Pane Layout** | **Left (~48%)**: Smooth windowed commit list with selection markers (`[✓]`) and edit flags (`✎`). Footer includes dynamic cursor tracking (e.g. `[5/51]`).<br>**Right (~52%)**: Metadata card showing Full SHA, Author, Email, Timestamps, Relative Age, and message quote block. |
| 🌿 **Organic Human Jitter** | Eliminates robotic mathematical steps and identical `:00` seconds with natural gap variance and randomized seconds. |
| 📅 **Multi-Day Active Days** | Partition commits across a chosen number of active days (e.g. 22 commits over 10 distinct days) during daytime hours. |
| 👤 **Interactive Author Editor** | Edit Author Name and Email for single commits or in batch across all selected commits with live form navigation. |
| ⏳ **Smart Time Range Picker** | Instant presets (*"Today Workday"*, *"Yesterday Workday"*, *"Past 3h"*, *"Past 8h"*) plus full custom date range controls. |
| 🛡️ **Non-Destructive Dry-Run** | Side-by-side diff inspection with **automatic safety backup branches** (`toolgit-backup-<timestamp>`) before any rewrite. |
| 🔄 **Async Rollback Engine** | Menu of previous safety backups with animated async loading overlay when restoring history. |
| ⚡ **Live Dev Toolchain** | Fast build script (`scripts/build/update.ps1`) and file watcher (`scripts/dev/watch.ps1`) for automatic hot rebuilding on save. |

---

## ⌨️ Keyboard Shortcuts

| Shortcut | Action | Description |
|---|---|---|
| <kbd>j</kbd> / <kbd>↓</kbd> | **Navigate Down** | Move cursor down with automatic windowed scrolling |
| <kbd>k</kbd> / <kbd>↑</kbd> | **Navigate Up** | Move cursor up with automatic windowed scrolling |
| <kbd>Space</kbd> | **Toggle Select** | Select or deselect the active commit for batch actions |
| <kbd>a</kbd> | **Select All** | Toggle selection across all commits in the repository |
| <kbd>e</kbd> | **Edit Author** | Open the interactive Author Name & Email editor modal |
| <kbd>d</kbd> | **Time Picker** | Open the Time Distribution & Active Days picker modal |
| <kbd>t</kbd> | **Quick 9-to-5** | Quick-distribute selected commits between 09:00 AM – 05:00 PM today |
| <kbd>w</kbd> | **Dry-Run Review** | Open the side-by-side diff review modal and confirm rewrite |
| <kbd>q</kbd> / <kbd>Esc</kbd> | **Quit / Back** | Close active modal or quit the application |
| <kbd>Ctrl+C</kbd> | **Force Exit** | Instant clean process shutdown |

---

## 🚀 Quick Start & Installation

### Prerequisites
- [Go 1.21+](https://go.dev/dl/)
- [Git](https://git-scm.com/) (available in system PATH)

### 1. Build and Install Globally
Run the automated update script to test, compile, and register `toolgit` into your terminal PATH:

```powershell
# In PowerShell:
.\scripts\build\update.ps1
```

*(Or in Windows Command Prompt: `.\scripts\build\update.cmd`)*

### 2. Launch Anywhere
Once installed, open any terminal in any Git repository and run:

```bash
toolgit
```

> [!TIP]
> If run outside a Git repository, `toolgit` automatically launches in **Mock Mode** (`🧪 Mock Mode`), allowing you to test all TUI features and algorithms safely without modifying any files.

---

## 🕹️ Basic Usage Tutorial

1. **Select Commits:** Use <kbd>↑</kbd> and <kbd>↓</kbd> (or <kbd>j</kbd>/<kbd>k</kbd>) to navigate the commit list. Press <kbd>Space</kbd> to select individual commits for batch modification, or <kbd>a</kbd> to toggle selection of all commits.
2. **Edit Author Metadata:** Press <kbd>e</kbd> to open the Author Editor modal. Here you can standardize the name and email for the selected commits.
3. **Time Shift (Jitter):** Press <kbd>d</kbd> to open the Time & Days Picker. Choose a predefined preset (e.g., "Today Workday") or enter custom dates. This will recalculate the timestamps for the selected commits using the Organic Jitter algorithm.
4. **Dry-Run & Commit:** Press <kbd>w</kbd> to enter Dry-Run mode. A side-by-side diff will appear displaying the original vs. rewritten commit graph. Confirm by typing "yes" to safely write the changes via Git plumbing.

---

## 📦 Release & Packaging Workflow

`toolgit` includes an automated GitHub Actions pipeline (`.github/workflows/release.yml`) for cross-platform distribution.

To trigger a new release:
1. Commit your changes and tag the commit with a version number (e.g., `v1.0.0`).
2. Push the tag to GitHub: `git push origin v1.0.0`

GitHub Actions will automatically:
- Build optimized, stripped binaries (`-s -w`) for Windows, macOS, and Linux (both `amd64` and `arm64`).
- Create a new GitHub Release.
- Upload the compiled binaries as release assets and automatically generate release notes.

*(Alternatively, for local packaging, you can run the included `.\scripts\build\release.ps1` script to cross-compile to a local `/release` folder).*

---

## 🌿 Organic Time Jitter & Active Days

Unlike naive linear interpolation ($\Delta t = \frac{\text{End} - \text{Start}}{N - 1}$) which creates an artificial grid of commits landing at identical seconds and midnight hours, `toolgit` uses **Organic Developer Simulation**:

```mermaid
graph LR
    A[Start: Apr 02] --> B[Day 1: 3 commits<br/>09:24, 11:42, 16:15]
    B --> C[Day 2: 2 commits<br/>14:08, 17:30]
    C --> D[Day 5: 4 commits<br/>10:11, 11:04, 15:48, 18:02]
    D --> E[...]
    E --> F[End: Apr 24]
```

- **Natural Gaps:** Intervals between commits vary dynamically from 20 minutes to 2+ hours.
- **Randomized Seconds:** Timestamps land on realistic natural seconds (e.g. `14:28:43`).
- **Active Days Constraint:** Spreads commits across your specified number of active days, preserving realistic coding days and rest intervals.

---

## 🛡️ Safety & O(1) Git Plumbing Engine

`toolgit` does **not** use fragile interactive git rebases. It leverages direct **Git plumbing** commands optimized into an **$O(1)$ native PowerShell process runspace**:

```mermaid
sequenceDiagram
    autonumber
    participant User as 👤 User
    participant Tool as 🛠️ toolgit
    participant PS as ⚡ PowerShell
    participant Git as 📦 Git Plumbing

    User->>Tool: Press 'w' (Review Dry-Run)
    Tool-->>User: Display Side-by-Side Metadata Diff
    User->>Tool: Confirm Rewrite ('Enter' / 'y')
    Tool->>Git: Create Backup (git branch toolgit-backup-<ts>)
    Tool->>PS: Execute Dynamic Script (O(1) Process Spawn)
    PS->>Git: Recreate Commit Trees (git commit-tree)
    PS->>Git: Atomically Update HEAD (git update-ref)
    Tool-->>User: Success Confirmation
```

> [!IMPORTANT]
> **Working Tree Safety:** Git plumbing updates commit graph references directly without touching files on disk. Your working tree remains clean and untouched.

---

## 🧪 Running the Test Suite

Unit, integration, and end-to-end repository workflow tests ensure mathematical accuracy, monotonicity, and plumbing safety:

```bash
go test -v ./...
```

---

## 📂 Project Architecture

```
toolgit/
├── cmd/toolgit/
│   └── main.go           # Minimal application entry point
├── internal/
│   ├── app/              # TUI components, state machines, and organic distribution engine
│   ├── core/             # Shared domain types (CommitState, DiffItem)
│   └── git/              # Git plumbing, commit loader, backup, and rewrite engine
├── scripts/
│   ├── build/            # PowerShell/Batch build and global installer scripts
│   └── dev/              # Background file watchers for hot reloading
├── docs/                 # Internal architecture and deep-dive design documents
├── go.mod                # Go module dependencies
├── README.md             # Project documentation
```

---

## 📚 Deep-Dive Documentation

- 👉 **[HOW_IT_WORKS.md](file:///e:/qa/toolgit/docs/HOW_IT_WORKS.md)**: Mathematical models for organic jitter, Git plumbing architecture, and viewport windowing algorithms.
- 👉 **[MULTI_AUTHOR_PROPOSAL.md](file:///e:/qa/toolgit/docs/MULTI_AUTHOR_PROPOSAL.md)**: Design proposal for team contributor rosters, author initials badges, pair-programming distribution, and GitHub `Co-authored-by:` trailers.
