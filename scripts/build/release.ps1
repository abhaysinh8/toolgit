# ToolGit Release & Packaging Script
$ErrorActionPreference = "Stop"

Write-Host "===============================================" -ForegroundColor Cyan
Write-Host "       📦 ToolGit Release Packager             " -ForegroundColor White
Write-Host "===============================================" -ForegroundColor Cyan

$ReleaseDir = "release"
if (Test-Path $ReleaseDir) {
    Remove-Item -Recurse -Force $ReleaseDir
}
New-Item -ItemType Directory -Path $ReleaseDir | Out-Null

$Targets = @(
    @{ OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{ OS = "windows"; Arch = "arm64"; Ext = ".exe" }
)

foreach ($Target in $Targets) {
    $os = $Target.OS
    $arch = $Target.Arch
    $ext = $Target.Ext
    
    $binName = "toolgit-$os-$arch$ext"
    $outPath = Join-Path $ReleaseDir $binName
    
    Write-Host "Building for $os/$arch..." -ForegroundColor Yellow
    
    # Set environment variables for cross-compilation
    $env:GOOS = $os
    $env:GOARCH = $arch
    
    # Build with ldflags to strip debug info and reduce size
    go build -ldflags "-s -w" -o $outPath ./cmd/toolgit
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "❌ Build failed for $os/$arch!" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "  ✓ Created $binName" -ForegroundColor Green
}

# Reset environment variables
$env:GOOS = ""
$env:GOARCH = ""

Write-Host "`n🎉 Release packaging complete! Binaries are in the '$ReleaseDir' directory." -ForegroundColor Green
Write-Host "===============================================`n" -ForegroundColor Cyan
