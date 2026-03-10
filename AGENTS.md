# Azure DevOps CLI - LLM First Guide

This CLI is optimized for AI assistants and LLM workflows. All commands default to YAML output with clean, structured data.

## Quick Start

```bash
# Configure your Azure DevOps profile
ado profile add --name myorg --org https://dev.azure.com/myorg --project myproject --pat YOUR_PAT

# List work items assigned to you (YAML output)
ado work-item list --mine

# Get PR details (YAML output)
ado pr list --repo myrepo --status active
```

## Core Concepts

### LLM-First Design
- **Default format**: YAML (not tables)
- **Clean field names**: `title` instead of `System.Title`
- **HTML stripped**: Descriptions are plain text
- **Token-optimized**: Only essential fields included

### Available Commands

#### Work Items
```bash
# List with filters
ado work-item list --mine                    # Your items
ado work-item list --state Active --type Bug # Filtered

# Get single item (includes comments automatically)
ado work-item get --id 12345

# Get with related QA Feedbacks
ado work-item get --id 12345 --related-full

# Show all fields (not LLM-optimized)
ado work-item get --id 12345 --full
```

#### Pull Requests
```bash
# List PRs
ado pr list --repo myrepo                    # Active PRs
ado pr list --repo myrepo --status all       # All PRs

# Show PR details
ado pr show --repo myrepo --pr-id 123

# Get full details with all fields
ado pr show --repo myrepo --pr-id 123 --full
```

## Output Formats

### YAML (Default)
Clean, human-readable, perfect for LLM consumption:
```yaml
id: 11376
title: Ajustar Nombre_entidad en selección de idiomas
state: Closed
type: QA Feedback
assigned_to: Nahuel Cioffi
description: |
  Modifico la selección de idiomas por otro.
has_comments: true
comment_count: 1
comments:
  - author: Romi Groisman
    date: 2025-03-27T17:02:29.147Z
    text: "arreglado"
```

### JSON
For programmatic parsing:
```bash
ado work-item get --id 12345 --format json
```

### Table
For human terminal viewing:
```bash
ado work-item list --format table
```

## Environment Variables

```bash
export AZURE_DEVOPS_ORG="https://dev.azure.com/myorg"
export AZURE_DEVOPS_PROJECT="myproject"
export AZURE_DEVOPS_PAT="your-personal-access-token"
export AZURE_DEVOPS_REPO="default-repo"
```

## Key Flags Reference

### Global Flags
- `-p, --profile`: Use specific profile
- `-f, --format`: Output format (yaml/json/table)

### Work Item Flags
- `--mine`: Items assigned to current user
- `--state`: Filter by state (Active, Closed, etc.)
- `--type`: Filter by type (Bug, Task, User Story, etc.)
- `--related-full`: Fetch and include QA Feedbacks
- `--full`: Show all original fields

### PR Flags
- `--repo`: Repository name (required)
- `--status`: Filter by status (active/completed/abandoned/all)
- `--pr-id`: Pull request ID
- `--full`: Show all original fields

## Profile Management

```bash
# Add a profile
ado profile add --name production --org https://dev.azure.com/company --project main --pat $PAT

# List profiles
ado profile list

# Switch default
ado profile set-default production

# Use specific profile for one command
ado work-item list --profile production
```

## Examples for Common Workflows

### Code Review Context
```bash
# Get PR with all context
ado pr show --repo backend --pr-id 456

# Check related work items
ado work-item get --id 11376 --related-full
```

### Sprint Planning
```bash
# All active items assigned to you
ado work-item list --mine --state Active

# All QA Feedbacks for a feature
ado work-item list --type "QA Feedback" --state Active
```

### Release Notes
```bash
# All completed items in a date range
ado work-item list --state Closed --format json
```

## Authentication

The CLI supports Azure DevOps Personal Access Tokens (PAT). Tokens are stored securely in your system's keyring.

To create a PAT:
1. Go to Azure DevOps → User Settings → Personal Access Tokens
2. Create new token with "Work Items" and "Code" read permissions
3. Use `ado profile add` to configure

## Troubleshooting

**Error: "organization not configured"**
→ Set `AZURE_DEVOPS_ORG` or use `--profile`

**Error: "project not configured"**  
→ Set `AZURE_DEVOPS_PROJECT` or use `--profile`

**Error: "PAT not configured"**
→ Run `ado profile add` to set up authentication

## Architecture

This CLI is designed specifically for:
- AI assistants analyzing development workflows
- LLM context windows (token-efficient output)
- Scripting and automation
- CI/CD pipeline integration

Unlike standard CLIs optimized for human readability, this tool prioritizes:
1. Structured data (YAML/JSON)
2. Relevant fields only
3. Clean text (no HTML)
4. Hierarchical relationships

## Version

Current version: v0.2.0 (LLM-First Release)

For updates: `git pull && go build -o ado ./cmd/main.go`
