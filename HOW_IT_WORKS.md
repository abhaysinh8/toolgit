# 🔍 How `toolgit` Works: Architecture & Internal Mechanics

This guide provides a comprehensive breakdown of `toolgit`'s internal architecture, mathematical distribution models, Git plumbing mechanics, and event loop lifecycle.

---

## 📖 Table of Contents
1. [🏗️ Model-View-Update (MVU) Architecture](#️-1-model-view-update-mvu-architecture)
2. [🧭 Git Discovery & Delimited Stream Parsing](#-2-git-discovery--delimited-stream-parsing)
3. [🌿 Organic Time Jitter & Active Days Engine](#-3-organic-time-jitter--active-days-engine)
4. [🖥️ Viewport Windowing & Layout System](#️-4-viewport-windowing--layout-system)
5. [🛡️ Non-Destructive Git Plumbing Engine](#️-5-non-destructive-git-plumbing-engine)
6. [🪟 Interactive Modal System](#-6-interactive-modal-system)
7. [💻 Development Toolchain & PATH Registration](#-7-development-toolchain--path-registration)

---

## 🏗️ 1. Model-View-Update (MVU) Architecture

`toolgit` is built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), following the Elm-style Model-View-Update paradigm:

```mermaid
graph TD
    subgraph EventLoop["🔄 Bubble Tea Event Loop"]
        Input["⌨️ Keypress / Terminal Resize"] --> Update["⚙️ Update(msg)"]
        Update --> State["📦 State Model Mutation"]
        State --> View["🎨 View()"]
        View --> Render["🖥️ Terminal ANSI Screen"]
    end

    subgraph Views["View Routing"]
        View --> MainSplit["Split-Pane Dashboard<br/>(48% Commits | 52% Details)"]
        View --> ModalOverlay["Centered Modal Dialogs<br/>(Author / Time / Dry-Run)"]
    end
```

---

## 🧭 2. Git Discovery & Delimited Stream Parsing

When `toolgit` initializes, it inspects the working directory:

```mermaid
sequenceDiagram
    participant CLI as 🛠️ toolgit
    participant Git as 📦 Local Git Engine

    CLI->>Git: git rev-parse --is-inside-work-tree
    alt In Git Repository
        Git-->>CLI: true
        CLI->>Git: git branch --show-current
        Git-->>CLI: "main" (or branch name)
        CLI->>Git: git log --format="%H%x1f%an%x1f%ae%x1f%aI%x1f%s%x1e" @{u}..HEAD
        Git-->>CLI: Unit-delimited commit stream
        CLI->>CLI: Parse into []*CommitState
    else Not in Git Repository
        Git-->>CLI: false
        CLI->>CLI: Load 5 mock commits with [🧪 Mock Mode] badge
    end
```

### Why ASCII Control Delimiters?
- **Unit Separator (`%x1f` / `0x1F`)**: Separates commit fields (Hash, Author, Email, ISO Timestamp, Subject).
- **Record Separator (`%x1e` / `0x1E`)**: Separates individual commits.
- **Benefit**: Eliminates parsing bugs caused by newlines, tabs, emojis, or multiline commit messages.

---

## 🌿 3. Organic Time Jitter & Active Days Engine

Naive distribution ($\Delta t = \frac{T_{\text{end}} - T_{\text{start}}}{N - 1}$) produces robotic `:00` seconds and slips into midnight sleep hours. `toolgit` replaces this with **Organic Developer Simulation**:

### A. Single-Day Ranges: Normalized Weight Jitter
For single-day intervals (e.g., `09:00 AM – 05:00 PM`):
1. **Weight Generation:** Generates $N-1$ random weights $w_i \in [0.5, 1.5]$.
2. **Normalized Duration:** 
   $$\Delta t_i = \frac{w_i}{\sum_{k=0}^{N-2} w_k} \cdot (T_{\text{end}} - T_{\text{start}})$$
3. **Micro-Seconds Jitter:** Adds $\pm 15$ seconds of random variance so timestamps land on natural seconds (e.g. `10:24:47`, `13:05:34`).
4. **Monotonicity Invariant:** Guarantees $t_i > t_{i-1} + 5\text{s}$ and clamps to $T_{\text{end}}$.

---

### B. Multi-Day Ranges: Active Days Partitioning
For multi-day intervals (e.g., April 2 to April 24):

```mermaid
graph LR
    subgraph Selection["1. Active Day Selection"]
        D1["Apr 02 (Start)"]
        D2["Apr 03"]
        D3["Apr 07"]
        D4["Apr 09"]
        D5["Apr 14"]
        D6["Apr 17"]
        D7["Apr 21"]
        D8["Apr 24 (End)"]
    end

    subgraph Partition["2. Commit Partitioning"]
        D1 --> C1["3 commits (Morning + Afternoon)"]
        D2 --> C2["2 commits (Workday)"]
        D3 --> C3["4 commits (Sprint Day)"]
        D4 --> C4["1 commit (Light Day)"]
        D5 --> C5["3 commits"]
        D6 --> C6["2 commits"]
        D7 --> C7["2 commits"]
        D8 --> C8["1 commit (Final)"]
    end
```

1. **Target Day Count ($D$):** Defaults to $\min(\text{CalendarDays}, \lceil N / 2.5 \rceil)$ or user-specified value.
2. **Fisher-Yates Day Selection:** Pins Start Day (0) and End Day, randomly selecting $D-2$ distinct days across the calendar window.
3. **Commit Partitioning:** Assigns $\ge 1$ commit per active day, distributing remainder randomly to create natural burst days (2–4 commits) and rest days.
4. **Working Hours Clamping:** Commits on each day fall strictly within active daytime hours (`09:30 – 18:30`).

---

## 🖥️ 4. Viewport Windowing & Layout System

To prevent screen overflow or text wrapping on repositories with 50+ commits:

| Calculation | Formula | Purpose |
|---|---|---|
| **Visible Capacity** | $\text{visibleRows} = \text{paneHeight} - 3$ | Available row count within left pane borders |
| **Window Start** | $\text{startIdx} = \max(0, \text{cursor} - \text{visibleRows} + 1)$ | Ensures active cursor is always visible |
| **Available Text Width** | $\text{availMsg} = \text{itemWidth} - 16$ | Space allocated for message preview |
| **Single-Line Truncation** | $\text{msg} = \text{msg}[:\text{availMsg}-1] + \text{"…"}$ | Guarantees zero multi-line word wrapping |

---

## 🛡️ 5. Non-Destructive O(1) Git Plumbing Engine

`toolgit` executes metadata rewrites directly through Git's low-level object database, optimized into an **$O(1)$ Process Execution Pipeline**:

```mermaid
graph TD
    A["1. Create Safety Backup<br/><code>git branch toolgit-backup-&lt;ts&gt;</code>"] --> B["2. Build Dynamic Script<br/>(Strings Builder in Go)"]
    B --> C["3. O(1) PowerShell Loop<br/><code>$PARENT = git commit-tree ...</code>"]
    C --> D["4. Environment Overrides<br/><code>$env:GIT_AUTHOR_NAME=...</code>"]
    D --> E["5. Atomically Move Ref<br/><code>git update-ref HEAD &lt;new_head_hash&gt;</code>"]
```

### ⚡ Batch Execution Architecture
Instead of spawning **2 `git` processes per commit** via Go's `exec.Command` (which is slow on Windows), `toolgit` dynamically builds a single native PowerShell script payload and executes it completely within a **single background process runspace**.

- **Multi-Line Messages:** Uses PowerShell Here-Strings (`@"\n ... \n"@`) to perfectly reconstruct commit bodies without variable escaping breaks.
- **Immediate Failure Traps:** Appends `$LASTEXITCODE` checks natively to short-circuit upon any `commit-tree` faults.

> [!NOTE]
> **Zero Working Tree Risk:** `git commit-tree` creates new commit objects without performing a checkout. Uncommitted files or working tree changes on disk are never touched.

---

## 🪟 6. Interactive Modal System

Modals render as centered floating cards using `lipgloss.Place`:

1. **Author & Email Editor (<kbd>e</kbd>)**:
   - Live form navigation with <kbd>Tab</kbd> / <kbd>Shift+Tab</kbd>.
   - Supports single commit or batch updates across all selected commits.
2. **Time & Active Days Picker (<kbd>d</kbd>)**:
   - Presets (*Today Workday*, *Yesterday Workday*, *Past 3h*, *Past 8h*).
   - Custom Range mode with Start Date, End Date, and Active Days fields.
   - Dynamic real-time preview of calendar days and active days distribution.
3. **Dry-Run Diff Review (<kbd>w</kbd>)**:
   - Color-coded side-by-side metadata diff table (`✎` modified flags in green/pink).
   - Safety backup notice and confirmation guard.
   - Asynchronous background execution with live loading spinner feedback.
4. **Safety Rollback (<kbd>r</kbd>)**:
   - Menu of timestamped automatic safety backup branches.
   - Restores branch synchronously inside a background goroutine with visual loading spinner.

---

## 💻 7. Development Toolchain & PATH Registration

| Script | Command | Purpose |
|---|---|---|
| **Fast Build & Update** | `.\update.ps1` | Runs test suite, compiles `toolgit.exe`, and updates global `go install` |
| **Skip-Tests Build** | `.\update.ps1 -SkipTests` | Rapid local compile and install |
| **File Watcher** | `.\watch.ps1` | Auto-detects `.go` file saves and updates global CLI in real time |
| **Batch Helper** | `.\update.cmd` | Command Prompt wrapper for Windows CMD |

Global CLI executable is registered at `%GOPATH%\bin\toolgit.exe` and is accessible anywhere from your system terminal.
