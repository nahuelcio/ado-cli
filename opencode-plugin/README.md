# @nahuelcio/opencode-ado

[![npm version](https://badge.fury.io/js/%40nahuelcio%2Fopencode-ado.svg)](https://www.npmjs.org/package/@nahuelcio/opencode-ado)

Azure DevOps plugin for **OpenCode** with server tools + TUI sidebar.

## Install

```bash
npx @nahuelcio/opencode-ado init
```

This wizard:
- stores your PAT in `~/.azure-devops-cli/pat`
- registers the server plugin in `~/.config/opencode/opencode.json`
- registers the sidebar plugin in `~/.config/opencode/tui.json`

## CLI

```bash
npx @nahuelcio/opencode-ado init       # interactive setup wizard
npx @nahuelcio/opencode-ado sync       # register existing ADO config in OpenCode + TUI
npx @nahuelcio/opencode-ado show       # show current config
npx @nahuelcio/opencode-ado --help     # help
```

For local workspace testing (without publishing):

```bash
node dist/bin/opencode-ado.js sync-local
```

## Configuration

The plugin config is stored in OpenCode as plugin options:

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

## Authentication

Requires an Azure DevOps PAT with scopes:
- Code (Read & Write)
- Pull Request Contribute (Read & Write)
- Work Items (Read)

PAT resolution order:
1. `AZURE_DEVOPS_PAT` environment variable
2. `~/.azure-devops-cli/pat`

## Available Tools

- `ado_prs`
- `ado_pr`
- `ado_pr_threads`
- `ado_pr_comment`
- `ado_review`
- `ado_pr_diff`
- `ado_pr_file`
- `ado_pr_review_context`
- `ado_select_pr`
- `ado_profile`
- `ado_profiles`
- `ado_profile_use`
- `ado_work_items`
- `ado_work_item`
- `ado_work_item_update`
- `ado_work_item_comment`
- `ado_work_item_types`
- `ado_related_work_items`

## Development

```bash
npm install
npm run build
npm test
```

## License

MIT

## Links

- [npm package](https://www.npmjs.org/package/@nahuelcio/opencode-ado)
- [GitHub repository](https://github.com/nahuelcio/azure-devops-cli-go)
- [Azure DevOps docs](https://learn.microsoft.com/azure/devops/)
