# Azure DevOps CLI (Go)

[![CI](https://github.com/your-org/azure-devops-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/your-org/azure-devops-cli/actions/workflows/ci.yml)
[![Release](https://github.com/your-org/azure-devops-cli/actions/workflows/release.yml/badge.svg)](https://github.com/your-org/azure-devops-cli/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/your-org/azure-devops-cli)](https://goreportcard.com/report/github.com/your-org/azure-devops-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

CLI tool for Azure DevOps - Manage work items and pull requests from the command line.

## Features

- **Profile Management**: Manage multiple Azure DevOps organizations/projects
- **Authentication**: Secure PAT storage using system keyring
- **Work Items**: List, get, create, update, add comments
- **Pull Requests**: List, view changes, threads, summaries
- **LLM Ready**: JSON output for AI assistants

## Installation

### Quick Install (Recommended)

#### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.sh | bash
```

With specific version:
```bash
curl -fsSL https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.sh | bash -s -- --version v1.0.0
```

#### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.ps1 | iex
```

With specific version:
```powershell
iwr -useb https://raw.githubusercontent.com/your-org/azure-devops-cli/main/install.ps1 | iex -Version v1.0.0
```

### Prerequisites

- Go 1.21 or higher (only for building from source)

### Build from source

```bash
# Clone the repository
git clone <repository-url>
cd azure-devops-cli-go

# Build the binary
go build -o ado ./cmd/main.go

# Or install to $GOPATH/bin
go install ./cmd/main.go
```

### Manual Installation

Download the latest release for your platform from the [Releases page](https://github.com/your-org/azure-devops-cli/releases).

**Supported platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)
- Windows (amd64)

Extract and move the binary to a directory in your PATH.

## Quick Start

### Option 1: Interactive Setup (Recommended)

```bash
# Run the setup wizard - it will guide you through everything
ado setup
```

This interactive wizard will ask for:
- Profile name (e.g., "myorg", "work")
- Organization URL (e.g., https://dev.azure.com/myorg)
- Project name
- Personal Access Token (PAT)
- Whether to set as default

### Option 2: Manual Setup

```bash
# 1. Add a profile
ado profile add --name myorg --org https://dev.azure.com/myorg --project myproject --default

# 2. Login with your PAT
ado auth login --profile myorg
# Enter your PAT when prompted

# 3. List work items
ado work-item list --profile myorg --state Active

# 4. Get a specific work item
ado work-item get --id 123 --profile myorg

# 5. List pull requests
ado pr list --profile myorg --repo myrepo
```

## Commands

### Quick Setup

```bash
# Interactive setup wizard (recommended for first-time users)
ado setup
```

### Profile Management

```bash
ado profile add --name <name> --org <org> --project <project> [--default]
ado profile list
ado profile show --name <name>
ado profile use --name <name>
ado profile delete --name <name>
```

### Authentication

```bash
ado auth login --profile <name> [--pat <token>]
ado auth logout --profile <name>
ado auth test --profile <name>
```

### Work Items

```bash
# List work items
ado work-item list [--state <state>] [--type <type>] [--format table|json|yaml]

# Get a work item
ado work-item get --id <id> [--format table|json|yaml]

# Create a work item
ado work-item create --title <title> --type <type> [--description <desc>]

# Add a comment
ado work-item comment --id <id> --text <text>

# Update state
ado work-item state --id <id> --state <state>
```

### Pull Requests

```bash
# List PRs
ado pr list --repo <repo> [--status active|completed|abandoned|all]

# Show PR details
ado pr show --repo <repo> --pr-id <id>

# View PR changes
ado pr changes --repo <repo> --pr-id <id>

# View PR threads/comments
ado pr threads --repo <repo> --pr-id <id>

# PR summary
ado pr summary --repo <repo> --pr-id <id>

# Review PR
ado pr review --repo <repo> --pr-id <id> --comment <text> [--status approved|rejected|waiting]
```

### Other Commands

```bash
# Show capabilities (JSON for LLMs)
ado capabilities

# Shell completion
ado autocomplete bash
ado autocomplete zsh
ado autocomplete fish

# Help
ado --help
ado <command> --help
```

## Configuration

Configuration is stored in `~/.azure-devops-cli/config.yaml`:

```yaml
version: 1
activeProfile: myorg
profiles:
  myorg:
    organization: https://dev.azure.com/myorg
    project: myproject
    auth:
      type: pat
      scopes:
        - vso.packaging
        - vso.code
        - vso.project
```

## Environment Variables

You can also use environment variables:

```bash
export AZURE_DEVOPS_ORG="https://dev.azure.com/myorg"
export AZURE_DEVOPS_PROJECT="myproject"
export AZURE_DEVOPS_PAT="your-pat-token"
```

## Security

- PATs are stored securely using the system keyring (Windows Credential Manager, macOS Keychain, Linux Secret Service)
- Fallback to encrypted file storage if keyring is unavailable
- Never commit your PAT to version control

## For LLM/AI Assistants

This CLI is designed to be used by AI assistants:

```bash
# Get capabilities in JSON format
ado capabilities

# Use JSON output for parsing
ado work-item list --format json
ado pr list --repo myrepo --format json
```

## Troubleshooting

### Go build errors

If you get import errors, make sure you're in the correct directory:

```bash
cd E:\workflow\azure-devops-cli-go
go mod tidy
go build -o ado ./cmd/main.go
```

### Authentication issues

If login fails:
1. Check your PAT has the required scopes
2. Verify the organization URL is correct
3. Try `ado auth test --profile <name>` to test connection

### Rate limiting

The CLI includes rate limiting. If you hit limits:
- Wait a few minutes between requests
- Use filters to reduce result sets

## CI/CD & Releases

### Automatic Releases

This project uses GitHub Actions to automatically build and release binaries for multiple platforms:

- **Linux**: amd64, arm64
- **macOS**: amd64, arm64 (Apple Silicon)
- **Windows**: amd64

### Release Process

1. Create and push a tag:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. GitHub Actions will automatically:
   - Build binaries for all platforms
   - Create a GitHub Release
   - Attach binaries to the release
   - Update install scripts

### Manual Build

```bash
# Local build
go build -o ado ./cmd/main.go

# Cross-compilation
# Linux
GOOS=linux GOARCH=amd64 go build -o ado-linux-amd64 ./cmd/main.go

# macOS
GOOS=darwin GOARCH=amd64 go build -o ado-darwin-amd64 ./cmd/main.go
GOOS=darwin GOARCH=arm64 go build -o ado-darwin-arm64 ./cmd/main.go

# Windows
GOOS=windows GOARCH=amd64 go build -o ado-windows-amd64.exe ./cmd/main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `go test ./...`
5. Submit a pull request

## License

MIT
