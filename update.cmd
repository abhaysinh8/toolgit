@echo off
setlocal
echo ===============================================
echo   ToolGit Build ^& Terminal Command Update
echo ===============================================

echo.
echo [1/3] Running test suite...
go test ./...
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Tests failed! Aborting update.
    exit /b %ERRORLEVEL%
)

echo.
echo [2/3] Building local toolgit.exe...
go build -o toolgit.exe .
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Local build failed!
    exit /b %ERRORLEVEL%
)

echo.
echo [3/3] Installing global terminal command...
go install .
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Failed to install globally!
    exit /b %ERRORLEVEL%
)

echo.
echo ===============================================
echo [SUCCESS] toolgit command is updated and ready!
echo You can run 'toolgit' in any terminal window.
echo ===============================================
