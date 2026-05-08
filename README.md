# Azure DevOps CLI - OpenCode Plugin

[![npm version](https://badge.fury.io/js/%40nahuelcio%2Fopencode-ado.svg)](https://www.npmjs.org/package/@nahuelcio/opencode-ado)

An OpenCode plugin that integrates Azure DevOps pull request workflows directly into your AI coding assistant.

## Features

- **PR Discovery**: List and search pull requests across your repositories
- **PR Details**: View full PR information including descriptions, commits, and work items
- **Review Management**: Approve or reject PRs with custom comments
- **Thread Comments**: Read and participate in code review discussions
- **Work Item Integration**: View linked work items and QA feedback
- **Multi-Profile Support**: Manage multiple organizations and projects
- **TUI Sidebar**: Visual panel showing PRs pending your review

## Installation

```bash
npx @nahuelcio/opencode-ado init
```

This will:
1. Prompt for your Azure DevOps organization URL
2. Securely store your Personal Access Token (PAT)
3. Configure profiles for your projects and repositories
4. Register the plugin with OpenCode

## Configuration

The plugin stores configuration in `~/.config/opencode/opencode.json`:

```jsonc
{
  "plugin": [
    [
      "@nahuelcio/opencode-ado",
      {
        "defaultProfile": "work",
        "profiles": {
          "work": {
            "org": "https://dev.azure.com/myorg",
            "project": "myproject",
            "patEnvVar": "AZURE_DEVOPS_PAT",
            "repos": ["backend", "frontend"],
            "default": true
          }
        }
      }
    ]
  ]
}
```

## Usage

### AI Commands

Once configured, the AI can use these tools:

- `ado_prs` - List active pull requests
- `ado_pr <repo> <id>` - Get details for a specific PR
- `ado_review <repo> <id> approve` - Approve a PR
- `ado_review <repo> <id> reject` - Reject a PR with a comment

### CLI Commands

```bash
# Interactive setup
npx @nahuelcio/opencode-ado init

# Sync existing configuration
npx @nahuelcio/opencode-ado sync

# Show current configuration
npx @nahuelcio/opencode-ado show

# Register local workspace build
node dist/bin/opencode-ado.js sync-local
```

## Authentication

The plugin requires an Azure DevOps Personal Access Token (PAT) with the following scopes:

- **Code**: Read & Write
- **Pull Request Contribute**: Read & Write
- **Work Items**: Read

Your PAT is stored in `~/.azure-devops-cli/pat` and never written to configuration files.

## Development

```bash
# Install dependencies
cd opencode-plugin
npm install

# Build
npm run build

# Test locally
node dist/bin/opencode-ado.js sync-local
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT

## Author

Nahuel Cioffi

## Links

- [npm Package](https://www.npmjs.org/package/@nahuelcio/opencode-ado)
- [GitHub Repository](https://github.com/nahuelcio/ado-cli)
- [Azure DevOps Documentation](https://learn.microsoft.com/azure/devops/)
