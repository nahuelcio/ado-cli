@echo off
REM Build script for Azure DevOps CLI (Windows)

echo Building Azure DevOps CLI...

REM Check if Go is installed
where go >nul 2>nul
if %errorlevel% neq 0 (
    echo ERROR: Go is not installed or not in PATH
    echo Please install Go from https://go.dev/dl/
    exit /b 1
)

REM Get Go version
for /f "tokens=3" %%i in ('go version') do set GO_VERSION=%%i
echo Go version: %GO_VERSION%

REM Download dependencies
echo Downloading dependencies...
go mod tidy
if %errorlevel% neq 0 (
    echo ERROR: Failed to download dependencies
    exit /b 1
)

REM Build the binary
echo Building binary...
go build -o ado.exe ./cmd/main.go
if %errorlevel% neq 0 (
    echo ERROR: Build failed
    exit /b 1
)

echo.
echo Build successful!
echo Binary created: ado.exe
echo.
echo Usage:
echo   ado.exe --help
echo.
pause
