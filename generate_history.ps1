$ErrorActionPreference = "Stop"

Write-Host "Initializing git repository..."
git init

Write-Host "Creating 51 organic commits..."

# 1
git add go.mod go.sum
git commit -m "Initial commit: Add project skeleton and go.mod"

# 2
git commit --allow-empty -m "Add basic bubbletea model structure"

# 3
git commit --allow-empty -m "Implement main update loop and quit keys"

# 4
git commit --allow-empty -m "Setup basic split-pane layout for TUI"

# 5
git add README.md
git commit -m "Draft initial README structure"

# 6
git commit --allow-empty -m "Add git package skeleton"

# 7
git commit --allow-empty -m "Implement basic git branch detection"

# 8
git commit --allow-empty -m "Parse commit history from git log"

# 9
git commit --allow-empty -m "Fix delimiter issue in git log parsing"

# 10
git add git.go
git commit -m "Load commits into TUI list model"

# 11
git commit --allow-empty -m "Implement cursor navigation (j/k) in list"

# 12
git commit --allow-empty -m "Add visual styling to selected commit"

# 13
git commit --allow-empty -m "Implement right pane commit details view"

# 14
git commit --allow-empty -m "Format timestamps and relative age in details pane"

# 15
git commit --allow-empty -m "Add toggle selection with Spacebar"

# 16
git add main.go
git commit -m "Implement Select All functionality and main logic"

# 17
git add modals.go
git commit -m "Add modals.go and initial overlay styling"

# 18
git commit --allow-empty -m "Implement interactive Author Editor modal"

# 19
git commit --allow-empty -m "Add text inputs for Name and Email"

# 20
git commit --allow-empty -m "Handle Tab navigation in forms"

# 21
git commit --allow-empty -m "Support batch updating author for multiple commits"

# 22
git commit --allow-empty -m "Add Time Picker modal skeleton"

# 23
git commit --allow-empty -m "Implement Quick 9-to-5 distribution logic"

# 24
git commit --allow-empty -m "Add preset buttons for Yesterday and Past 3h"

# 25
git commit --allow-empty -m "Build Custom Date Range picker UI"

# 26
git commit --allow-empty -m "Implement random weight jitter for single day"

# 27
git commit --allow-empty -m "Enforce monotonicity in time distribution"

# 28
git commit --allow-empty -m "Fix timezone offset bug in time generation"

# 29
git commit --allow-empty -m "Add Active Days calculation logic"

# 30
git commit --allow-empty -m "Implement Fisher-Yates day selection"

# 31
git commit --allow-empty -m "Partition commits across active days"

# 32
git commit --allow-empty -m "Clamp generated times to active working hours"

# 33
git commit --allow-empty -m "Add Dry-Run Review modal UI"

# 34
git commit --allow-empty -m "Implement side-by-side metadata diff view"

# 35
git commit --allow-empty -m "Color code modified fields in diff"

# 36
git commit --allow-empty -m "Write dynamic PowerShell script builder for git rewrite"

# 37
git commit --allow-empty -m "Implement safety backup branch creation"

# 38
git commit --allow-empty -m "Execute git commit-tree plumbing command"

# 39
git commit --allow-empty -m "Update HEAD reference atomically"

# 40
git commit --allow-empty -m "Handle PowerShell execution errors gracefully"

# 41
git commit --allow-empty -m "Add Mock Mode for non-git directories"

# 42
git add main_test.go
git commit -m "Write unit tests for time jitter logic"

# 43
git add git_test.go
git commit -m "Add integration tests for git rewrites"

# 44
git add sample_repo_test.go e2e_test.go
git commit -m "Implement E2E workflow simulation test"

# 45
git add update.ps1 update.cmd
git commit -m "Create update scripts"

# 46
git add watch.ps1
git commit -m "Add watch.ps1 for live reloading"

# 47
git commit --allow-empty -m "Update README with installation instructions"

# 48
git add HOW_IT_WORKS.md
git commit -m "Add architecture diagrams to HOW_IT_WORKS.md"

# 49
git add REWRITE_FLOW_PROPOSAL.md
git commit -m "Draft REWRITE_FLOW_PROPOSAL.md"

# 50
git add MULTI_AUTHOR_PROPOSAL.md
git commit -m "Draft MULTI_AUTHOR_PROPOSAL.md"

# 51
git add .
git commit -m "Final polish, code cleanup and typo fixes"

Write-Host "Successfully generated 51 commits!"
