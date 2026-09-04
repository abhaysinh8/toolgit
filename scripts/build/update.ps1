# ToolGit Fast Rebuild & Terminal Command Updater
param (
    [switch]$SkipTests = $false
)

$ErrorActionPreference = "Stop"

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "     ToolGit Build & Terminal Command Update   " -ForegroundColor White
Write-Host "===============================================" -ForegroundColor Cyan

# 1. Run Tests (if not skipped)
if (-not $SkipTests) {
    Write-Host "`n[1/3] Running test suite..." -ForegroundColor Yellow
    go test ./...
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[-] Tests failed! Aborting update." -ForegroundColor Red
        exit 1
    }
    Write-Host "  [+] All tests passed." -ForegroundColor Green
} else {
    Write-Host "`n[1/3] Skipping tests..." -ForegroundColor Gray
}

# 2. Local Workspace Build
Write-Host "`n[2/3] Building local executable (toolgit.exe)..." -ForegroundColor Yellow
go build -o toolgit.exe ./cmd/toolgit
if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Build failed!" -ForegroundColor Red
    exit 1
}
Write-Host "  [+] Local binary created: .\toolgit.exe" -ForegroundColor Green

# 3. Global Terminal Command Update
Write-Host "`n[3/3] Updating global terminal command (toolgit)..." -ForegroundColor Yellow
go install ./cmd/toolgit
if ($LASTEXITCODE -ne 0) {
    Write-Host "[-] Failed to install globally!" -ForegroundColor Red
    exit 1
}

$goBin = (go env GOPATH) + "\bin\toolgit.exe"
Write-Host "  [+] Global CLI installed to: $goBin" -ForegroundColor Green

Write-Host "`nSuccess! You can now run 'toolgit' directly in any terminal." -ForegroundColor Green
Write-Host "===============================================`n" -ForegroundColor Cyan
