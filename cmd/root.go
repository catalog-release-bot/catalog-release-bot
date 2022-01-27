package cmd

import (
	"github.com/spf13/cobra"
)

func Execute() error {
	rootCmd := cobra.Command{
		Use:   "catalog-release-bot",
		Short: "Automate releases of catalogs for OLM",
		Long:  "Automate releases of catalogs for Operator Lifecycle Manager",
	}

	rootCmd.AddCommand(newActionCmd())
	rootCmd.AddCommand(newWebhookCmd())
	return rootCmd.Execute()
}
