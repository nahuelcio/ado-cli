# Contributing to Azure DevOps CLI

Thank you for your interest in contributing to Azure DevOps CLI! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- Go 1.21 or higher
- Git
- Azure DevOps account (for testing)

### Local Development

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/azure-devops-cli.git
   cd azure-devops-cli
   ```

3. Install dependencies:
   ```bash
   go mod download
   ```

4. Build the project:
   ```bash
   go build -o ado ./cmd/main.go
   ```

5. Run tests:
   ```bash
   go test ./...
   ```

## Project Structure

```
azure-devops-cli-go/
├── cmd/main.go              # Entry point
├── internal/
│   ├── api/                 # Azure DevOps API clients
│   ├── cli/                 # CLI commands (Cobra)
│   ├── auth/                # Authentication & credentials
│   └── config/              # Configuration management
├── .github/workflows/       # GitHub Actions
├── install.sh               # Linux/macOS installer
├── install.ps1              # Windows installer
└── README.md                # Documentation
```

## Making Changes

### Adding a New Command

1. Create a new file in `internal/cli/<command>.go`
2. Define the cobra command
3. Add it to `root.go` with `rootCmd.AddCommand()`
4. Update `capabilities.go` if needed
5. Add documentation to README.md

### Code Style

- Follow standard Go conventions
- Use `gofmt` to format code
- Add comments for exported functions
- Keep functions focused and small

### Testing

Before submitting a PR:

1. Run all tests:
   ```bash
   go test ./...
   ```

2. Check formatting:
   ```bash
   gofmt -l .
   ```

3. Build for all platforms:
   ```bash
   make build
   ```

4. Test the CLI locally:
   ```bash
   ./ado --help
   ./ado setup
   ```

## Submitting Changes

1. Create a new branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make your changes and commit:
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

3. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```

4. Create a Pull Request on GitHub

### Commit Message Convention

We follow conventional commits:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code style changes (formatting)
- `refactor:` Code refactoring
- `test:` Adding/updating tests
- `chore:` Maintenance tasks

Example:
```
feat: add work-item update command

Add ability to update work item fields and state
through the CLI using the update subcommand.
```

## Testing with Real Azure DevOps

To test with a real Azure DevOps instance:

1. Create a test organization (or use an existing one)
2. Generate a PAT with required scopes
3. Run setup:
   ```bash
   ./ado setup
   ```
4. Test commands:
   ```bash
   ./ado work-item list --profile test
   ```

## Questions?

- Open an issue for bugs or feature requests
- Start a discussion for questions
- Check existing issues before creating new ones

## Code of Conduct

- Be respectful and constructive
- Welcome newcomers
- Focus on what's best for the community

Thank you for contributing!
