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
	Short: "Azure DevOps CLI",
	Long:  `A CLI tool for interacting with Azure DevOps.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
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

	viper.BindPFlag("profile", rootCmd.PersistentFlags().Lookup("profile"))
	viper.BindPFlag("format", rootCmd.PersistentFlags().Lookup("format"))

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
	Version   string                 `json:"version"`
	Commands  map[string]CommandInfo `json:"commands"`
	GlobalOpts GlobalOpts            `json:"globalOptions"`
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
	Short: "Show CLI capabilities in JSON format",
	Long:  `Outputs a JSON representation of all available commands and options for LLM consumption.`,
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
