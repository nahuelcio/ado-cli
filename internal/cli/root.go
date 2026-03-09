package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "ado",
	Short: "Azure DevOps CLI - Manage work items and pull requests",
	Long: `Azure DevOps CLI (ado) - A command-line interface for Azure DevOps

DESCRIPTION:
  ado is a CLI tool for interacting with Azure DevOps services. It provides
  comprehensive commands to manage work items, pull requests, and authentication.
  The tool supports multiple profiles for different organizations/projects and
  offers output in multiple formats (table, JSON, YAML) suitable for both human
  users and programmatic consumption.

MAIN CATEGORIES:
  • Profile Management    Configure and switch between Azure DevOps organizations
  • Authentication        Secure PAT-based authentication with keyring storage
  • Work Items            Create, read, update work items; add comments; change states
  • Pull Requests         List, review, and manage PRs with threads and changes

GETTING STARTED:
  First time setup:
    $ ado setup                          # Interactive configuration wizard

  Or manual setup:
    $ ado profile add --name myorg --org https://dev.azure.com/myorg --project myproject
    $ ado auth login --profile myorg     # Enter your PAT when prompted

  Basic usage:
    $ ado work-item list --state Active
    $ ado work-item get --id 123
    $ ado pr list --repo myrepo

GLOBAL FLAGS:
  --profile, -p     Use a specific profile (default: active profile)
  --format, -f      Output format: table, json, yaml (default: table)
  --config          Config file path (default: $HOME/.azure-devops-cli.yaml)

ENVIRONMENT VARIABLES:
  AZURE_DEVOPS_ORG         Default organization URL
  AZURE_DEVOPS_PROJECT     Default project name
  AZURE_DEVOPS_PAT         Personal Access Token (use with caution)

FOR LLMs AND AUTOMATION:
  Get structured command information:
    $ ado capabilities                   # JSON output of all commands
  
  Use JSON output for parsing:
    $ ado work-item list --format json
    $ ado pr list --repo myrepo --format json

EXAMPLES:
  # Work with work items
  ado work-item list --state "Active" --type "Task"
  ado work-item create --title "Fix bug" --type Bug --description "Details here"
  ado work-item comment --id 123 --text "Updated the implementation"
  ado work-item state --id 123 --state "Resolved"

  # Work with pull requests
  ado pr list --repo myrepo --status active
  ado pr show --repo myrepo --pr-id 456
  ado pr changes --repo myrepo --pr-id 456
  ado pr threads --repo myrepo --pr-id 456

  # Profile and authentication management
  ado profile list
  ado auth test --profile myorg
  ado auth logout --profile myorg

Learn more about a command:
  ado <command> --help
  ado work-item --help
  ado pr --help`,
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.azure-devops-cli.yaml)")
	rootCmd.PersistentFlags().StringP("profile", "p", "", "Azure DevOps profile to use")
	rootCmd.PersistentFlags().StringP("format", "f", "table", "Output format (table/json/yaml)")

	_ = viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
	_ = viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))

	rootCmd.AddCommand(profileCmd)
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(workItemCmd)
	rootCmd.AddCommand(prCmd)
	rootCmd.AddCommand(capabilitiesCmd)
	rootCmd.AddCommand(autocompleteCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".azure-devops-cli")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

type Capabilities struct {
	Version    string                 `json:"version"`
	Commands   map[string]CommandInfo `json:"commands"`
	GlobalOpts GlobalOpts             `json:"globalOptions"`
}

type CommandInfo struct {
	Use         string   `json:"use"`
	Short       string   `json:"short"`
	Subcommands []string `json:"subcommands,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

type GlobalOpts struct {
	Profile string   `json:"profile"`
	Format  []string `json:"format"`
}

func GetCapabilities() Capabilities {
	return Capabilities{
		Version: "1.0.0",
		GlobalOpts: GlobalOpts{
			Profile: "Azure DevOps profile to use",
			Format:  []string{"table", "json", "yaml"},
		},
		Commands: map[string]CommandInfo{
			"setup": {
				Use:   "setup",
				Short: "Interactive setup wizard for first-time configuration",
				Examples: []string{
					"ado setup",
				},
			},
			"profile": {
				Use:         "profile",
				Short:       "Manage Azure DevOps profiles",
				Subcommands: []string{"add", "list", "delete", "show", "use"},
				Examples: []string{
					"ado profile add --name myorg --org https://dev.azure.com/myorg --project myproject",
					"ado profile list",
					"ado profile use myorg",
				},
			},
			"auth": {
				Use:         "auth",
				Short:       "Manage authentication",
				Subcommands: []string{"login", "logout", "test"},
				Examples: []string{
					"ado auth login --profile myorg",
					"ado auth test --profile myorg",
				},
			},
			"work-item": {
				Use:         "work-item",
				Short:       "Manage Azure DevOps work items",
				Subcommands: []string{"list", "get", "create", "comment", "field", "state", "update"},
				Examples: []string{
					"ado work-item list --format table",
					"ado work-item get --id 123 --format json",
					"ado work-item create --title \"New Task\" --type Task",
				},
			},
			"pr": {
				Use:         "pr",
				Short:       "Manage Azure DevOps pull requests",
				Subcommands: []string{"list", "show", "changes", "threads", "summary", "review"},
				Examples: []string{
					"ado pr list --repo myrepo",
					"ado pr show --repo myrepo --pr-id 123",
					"ado pr review --repo myrepo --pr-id 123 --comment \"LGTM\"",
				},
			},
		},
	}
}

var capabilitiesCmd = &cobra.Command{
	Use:   "capabilities",
	Short: "Show CLI capabilities in JSON format for LLM integration",
	Long: `Output a JSON representation of all available commands and options.

This command is designed for LLMs and automation tools to programmatically 
understand the CLI structure, available commands, and their usage patterns.

The JSON output includes:
- CLI version
- All available commands with descriptions
- Subcommands and examples
- Global flags and options

For Humans:
  Use 'ado --help' or 'ado <command> --help' for readable help text.

For LLMs/Automation:
  Use this command to get structured information about all CLI capabilities.
  This helps LLMs construct valid commands and understand the tool's functionality.

Example output structure:
  {
    "version": "1.0.0",
    "commands": {
      "work-item": {
        "use": "work-item",
        "short": "Manage work items",
        "subcommands": ["list", "get", "create"],
        "examples": ["ado work-item list --state Active"]
      }
    },
    "globalOptions": {
      "profile": "Azure DevOps profile to use",
      "format": ["table", "json", "yaml"]
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		capabilities := GetCapabilities()
		output, err := json.MarshalIndent(capabilities, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal capabilities: %w", err)
		}
		fmt.Println(string(output))
		return nil
	},
}
