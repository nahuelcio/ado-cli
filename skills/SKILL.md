---
name: azure-devops-cli
description: Interface with Azure DevOps to manage work items, pull requests, and development workflows. Use when the user wants to check work items assigned to them, view QA feedbacks, review pull requests, get work item details with comments, or analyze development status. This skill is LLM-first optimized with YAML output format and clean field names.
---

# Azure DevOps CLI Skill

This skill provides seamless integration with Azure DevOps for managing development workflows, optimized for LLM consumption with clean, structured data output.

## When to Use

Use this skill when the user wants to:
- Check work items assigned to them or the team
- View details of specific work items including comments and QA feedbacks
- List or review pull requests
- Get development context for decision-making
- Analyze project status or sprint progress
- Find related work items, bugs, or QA feedbacks

## Quick Start

```bash
# List your assigned work items
ado work-item list --mine

# Get a specific work item with comments
ado work-item get --id 12345

# Get work item with all related QA feedbacks
ado work-item get --id 12345 --related-full

# List active pull requests
ado pr list --repo myrepo

# Show PR details
ado pr show --repo myrepo --pr-id 456
```

## Core Commands

### Work Items

**List work items:**
```bash
# Your items
ado work-item list --mine

# Filtered by state and type
ado work-item list --state Active --type Bug

# Items assigned to someone
ado work-item list --assigned-to "John Doe"
```

**Get single work item:**
```bash
# Basic info with comments (if any)
ado work-item get --id 12345

# With related QA Feedbacks
ado work-item get --id 12345 --related-full

# Full details (all fields)
ado work-item get --id 12345 --full
```

**Output format (default: YAML):**
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
related_count: 11
related:
  - type: parent
    id: "11262"
    url: https://...
```

### Pull Requests

**List PRs:**
```bash
ado pr list --repo myrepo                    # Active PRs
ado pr list --repo myrepo --status all       # All PRs
ado pr list --repo myrepo --status completed # Completed
```

**Show PR details:**
```bash
ado pr show --repo myrepo --pr-id 123
```

**Show PR diff (file changes metadata):**
```bash
# Show diff with file metadata (all files)
ado pr diff --repo myrepo --pr-id 123

# Limit to first 5 files
ado pr diff --repo myrepo --pr-id 123 --max-files 5

# Get in JSON format
ado pr diff --repo myrepo --pr-id 123 --format json
```

**Output format for diff:**
```yaml
pullRequestId: 3251
title: "Fix: mejorar manejo de errores"
sourceBranch: refs/heads/feature/error-handling
targetBranch: refs/heads/main
totalFiles: 3
totalAdditions: 45
totalDeletions: 12
files:
  - path: "/src/auth/LoginService.cs"
    changeType: edit
    originalPath: ""
    additions: 20
    deletions: 5
  - path: "/src/auth/TokenValidator.cs"
    changeType: add
    originalPath: ""
    additions: 25
    deletions: 0
  - path: "/src/utils/OldHelper.cs"
    changeType: delete
    originalPath: ""
    additions: 0
    deletions: 12
```

**Note:** The diff command shows file metadata (what changed) but not the actual code content. 
Azure DevOps API returns file lists and statistics, but full diff content requires fetching 
individual files separately or using the web interface.

**Output format:**
```yaml
id: 123
title: "Fix authentication bug"
status: active
source_branch: feature/auth-fix
target_branch: main
author: John Doe
description: "Fixes the login issue..."
merge_status: succeeded
reviewers:
  - name: Jane Smith
    vote: approved
  - name: Bob Wilson
    vote: approved_with_suggestions
reviewer_count: 2
```

## Key Features

### LLM-First Design
- **Default format**: YAML (not tables)
- **Clean field names**: `title` instead of `System.Title`
- **HTML stripped**: Descriptions are plain text
- **Token-optimized**: Only essential fields included

### Automatic Inclusions
- Comments are fetched automatically if they exist
- Related work items are listed with type and URL
- QA Feedbacks can be fetched with `--related-full` flag

### Output Formats
- `yaml` (default): Human-readable, perfect for LLMs
- `json`: For programmatic parsing
- `table`: For human terminal viewing

## Configuration

### Environment Variables
```bash
export AZURE_DEVOPS_ORG="https://dev.azure.com/myorg"
export AZURE_DEVOPS_PROJECT="myproject"
export AZURE_DEVOPS_PAT="your-personal-access-token"
export AZURE_DEVOPS_REPO="default-repo"
```

### Profile Management
```bash
# Add a profile
ado profile add --name production --org https://dev.azure.com/company --project main --pat $PAT

# List profiles
ado profile list

# Use specific profile
ado work-item list --profile production
```

## Common Workflows

### Code Review Context
```bash
# Get PR with full context
ado pr show --repo backend --pr-id 456

# Check related work items with QA feedbacks
ado work-item get --id 11376 --related-full
```

### Sprint Planning
```bash
# All active items assigned to you
ado work-item list --mine --state Active

# All QA Feedbacks for a feature
ado work-item list --type "QA Feedback" --state Active
```

### Bug Analysis
```bash
# Get bug details with all context
ado work-item get --id 12345 --related-full

# Check if there are related QA feedbacks
```

## Flags Reference

### Global
- `-p, --profile`: Use specific profile
- `-f, --format`: Output format (yaml/json/table)

### Work Items
- `--mine`: Items assigned to current user
- `--state`: Filter by state (Active, Closed, etc.)
- `--type`: Filter by type (Bug, Task, User Story, QA Feedback, etc.)
- `--related-full`: Fetch and include QA Feedbacks with full details
- `--full`: Show all original fields (not LLM-optimized)

### Pull Requests
- `--repo`: Repository name (required)
- `--status`: Filter by status (active/completed/abandoned/all)
- `--pr-id`: Pull request ID
- `--full`: Show all original fields

## Troubleshooting

**Error: "organization not configured"**
→ Set `AZURE_DEVOPS_ORG` or use `--profile`

**Error: "project not configured"**
→ Set `AZURE_DEVOPS_PROJECT` or use `--profile`

**Error: "PAT not configured"**
→ Run `ado profile add` to set up authentication

## Examples

### Example 1: Get work item context
Input: "What's the status of work item 11376?"
```bash
ado work-item get --id 11376
```
Output: YAML with title, state, assigned_to, description, comments, and related items

### Example 2: Check my tasks
Input: "Show me my active work items"
```bash
ado work-item list --mine --state Active
```
Output: List of active items with id, title, state, type, assigned_to

### Example 3: Review PR
Input: "What's happening with PR 456 in the backend repo?"
```bash
ado pr show --repo backend --pr-id 456
```
Output: PR details with title, status, author, reviewers, and description

### Example 4: Get QA feedbacks
Input: "Get work item 11376 and show me all related QA feedbacks"
```bash
ado work-item get --id 11376 --related-full
```
Output: Work item details plus qa_feedbacks array with id, title, state, description

## Notes

- This CLI is specifically designed for AI assistants and LLM workflows
- All commands prioritize structured data (YAML/JSON) over human-readable tables
- HTML is automatically stripped from descriptions for cleaner output
- The `--related-full` flag is particularly useful for getting complete context on work items with many related QA feedbacks
- Use `--full` flag only when you need all the raw Azure DevOps fields
