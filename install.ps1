# Azure DevOps CLI Installer for Windows
# Usage: iwr -useb https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.ps1 | iex

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\ado",
    [switch]$AddToPath = $true
)

$ErrorActionPreference = "Stop"
$Repo = "nahuelcio/ado-cli"
$BinaryName = "ado.exe"

# Colors for output
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Success($message) {
    Write-ColorOutput Green "✓ $message"
}

function Write-Error($message) {
    Write-ColorOutput Red "✗ $message"
}

function Write-Warning($message) {
    Write-ColorOutput Yellow "! $message"
}

# Detect architecture
function Get-Architecture {
    $arch = (Get-WmiObject -Class Win32_Processor).Architecture
    switch ($arch) {
        0 { return "amd64" }  # x86
        9 { return "amd64" }  # x64
        12 { return "arm64" } # ARM64
        default { 
            Write-Error "Unsupported architecture: $arch"
            exit 1
        }
    }
}

# Get latest version
function Get-LatestVersion {
    if ($Version -eq "latest") {
        Write-Host "Fetching latest version..."
        try {
            $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
            $script:Version = $release.tag_name
        }
        catch {
            Write-Error "Could not fetch latest version"
            exit 1
        }
    }
    Write-Success "Installing $BinaryName $Version"
}

# Download and install
function Install-Binary {
    $arch = Get-Architecture
    $target = "windows-amd64"
    $downloadUrl = "https://github.com/$Repo/releases/download/$Version/ado-$target.zip"
    $tempDir = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())
    
    Write-Host "Downloading from $downloadUrl..."
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile "$tempDir\$BinaryName.zip"
    }
    catch {
        Write-Error "Failed to download: $_"
        exit 1
    }
    
    Write-Host "Extracting..."
    Expand-Archive -Path "$tempDir\$BinaryName.zip" -DestinationPath $tempDir -Force
    
    Write-Host "Installing to $InstallDir..."
    if (!(Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    $sourcePath = Join-Path $tempDir $BinaryName
    $destPath = Join-Path $InstallDir $BinaryName
    
    # Close any running instances
    $process = Get-Process -Name "ado" -ErrorAction SilentlyContinue
    if ($process) {
        Write-Warning "Closing running ado process..."
        $process | Stop-Process -Force
        Start-Sleep -Seconds 1
    }
    
    Move-Item -Path $sourcePath -Destination $destPath -Force
    Remove-Item -Path $tempDir -Recurse -Force
    
    # Add to PATH if requested
    if ($AddToPath) {
        Add-ToPath
    }
}

# Add to PATH
function Add-ToPath {
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    
    if ($currentPath -notlike "*$InstallDir*") {
        Write-Host "Adding to PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "User")
        $env:Path = "$env:Path;$InstallDir"
        Write-Success "Added $InstallDir to PATH"
    }
    else {
        Write-Host "Already in PATH"
    }
}

# Verify installation
function Verify-Installation {
    $binaryPath = Join-Path $InstallDir $BinaryName
    
    if (Test-Path $binaryPath) {
        try {
            $installedVersion = & $binaryPath --version 2>$null
            if (!$installedVersion) {
                $installedVersion = "unknown"
            }
        }
        catch {
            $installedVersion = "unknown"
        }
        
        Write-Success "Successfully installed $BinaryName"
        Write-Host "  Version: $installedVersion"
        Write-Host "  Location: $binaryPath"
        Write-Host ""
        Write-Host "Quick start:"
        Write-Host "  ado --help"
        Write-Host "  ado profile add --name myorg --org https://dev.azure.com/myorg --project myproject"
        Write-Host "  ado auth login --profile myorg"
        Write-Host ""
        
        if ($AddToPath) {
            Write-Host "NOTE: Please restart your terminal or run 'refreshenv' to use the 'ado' command"
        }
    }
    else {
        Write-Error "Installation failed - binary not found"
        exit 1
    }
}

# Main
function Main {
    Write-Host "Azure DevOps CLI Installer for Windows"
    Write-Host "======================================"
    Write-Host ""
    
    Get-LatestVersion
    Install-Binary
    Verify-Installation
}

# Show help
if ($args -contains "--help" -or $args -contains "-h") {
    Write-Host "Usage: install.ps1 [OPTIONS]"
    Write-Host ""
    Write-Host "Options:"
    Write-Host "  -Version <version>      Install specific version (default: latest)"
    Write-Host "  -InstallDir <dir>       Installation directory (default: %LOCALAPPDATA%\Programs\ado)"
    Write-Host "  -AddToPath              Add to PATH (default: true)"
    Write-Host "  -Help                   Show this help message"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host '  iwr -useb https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.ps1 | iex'
    Write-Host '  iwr -useb ... | iex -Version v1.0.0'
    Write-Host '  iwr -useb ... | iex -InstallDir "C:\Tools"'
    exit 0
}

Main
