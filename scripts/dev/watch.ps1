# ToolGit Auto-Rebuild Watcher
# Automatically tests, builds, and updates 'toolgit' in your terminal whenever you save any .go file!

Write-Host "══════════════════════════════════════════════════════════" -ForegroundColor Cyan
Write-Host "  👀  ToolGit File Watcher Active (Listening for saves)   " -ForegroundColor White
Write-Host "  Press Ctrl+C to stop watching.                          " -ForegroundColor Gray
Write-Host "══════════════════════════════════════════════════════════" -ForegroundColor Cyan

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$updateScript = Join-Path $repoRoot "scripts\build\update.ps1"

$watcher = New-Object System.IO.FileSystemWatcher
$watcher.Path = $repoRoot
$watcher.Filter = "*.go"
$watcher.IncludeSubdirectories = $true
$watcher.EnableRaisingEvents = $true

$lastRun = [DateTime]::MinValue

while ($true) {
    $result = $watcher.WaitForChanged([System.IO.WatcherChangeTypes]::Changed -bor [System.IO.WatcherChangeTypes]::Created, 1000)
    if ($result.TimedOut -eq $false) {
        # Debounce multiple events within 1.5 seconds
        $now = [DateTime]::Now
        if (($now - $lastRun).TotalSeconds -gt 1.5) {
            $lastRun = $now
            Write-Host "`n[Change detected in $($result.Name)] 🔄 Rebuilding and updating..." -ForegroundColor Magenta
            Push-Location $repoRoot
            try {
                & $updateScript -SkipTests
            } finally {
                Pop-Location
            }
        }
    }
}
