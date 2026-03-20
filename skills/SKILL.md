---
name: azure-devops-cli
description: >
  Manage Azure DevOps work items and pull requests.
  Use for: work items, PRs, code review, sprint planning.
  LLM-first: YAML output, clean fields, no HTML.
---

# Azure DevOps CLI

## Setup
```bash
ado setup
ado profile add --name myorg --org https://dev.azure.com/myorg --project myproject --pat YOUR_PAT
```

## Work Items
```bash
ado work-item list --mine                              # my items
ado work-item list --state Active --type Bug          # filtered
ado work-item get --id 12345                         # detail + comments
ado work-item get --id 12345 --related-full          # + QA Feedbacks
ado work-item create --title "Bug" --type Bug         # create
ado work-item state --id 123 --state Resolved         # change state
ado work-item comment --id 123 --text "Fixed"         # comment
```

## Pull Requests
```bash
ado pr list --repo myrepo                              # active PRs
ado pr list --repo myrepo --status all                # all PRs
ado pr show --repo myrepo --pr-id 456                 # detail
ado pr threads --repo myrepo --pr-id 456              # discussions
ado pr review --repo myrepo --pr-id 456 --status approved   # vote
ado pr review --repo myrepo --pr-id 456 --comment "LGTM!"  # comment
```

## Output
```bash
--format yaml    # default (LLM-optimized)
--format json   # scripts
--format table  # terminal
```

## Profiles
```bash
ado profile list
--profile myorg   # use specific
```

## YAML Output Example
```yaml
# Work Item
id: 123
title: "Fix login bug"
state: Active
type: Bug
assigned_to: Juan
has_comments: true
related:
  - type: child
    id: "456"

# PR
id: 123
title: "Fix auth"
status: active
author: Juan
reviewers:
  - "✓ Juan"
  - "✗ Pedro"
```
