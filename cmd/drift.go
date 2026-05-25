package cmd

import (
	_ "embed"

	"github.com/Use-Tusk/tusk-cli/internal/utils"
	"github.com/spf13/cobra"
)

const (
	configFlagUsage         = "config file (default is .tusk/config.yaml)"
	configOverrideFlagUsage = "config override file (merges on top of the base config)"
)

//go:embed short_docs/drift/drift_overview.md
var driftOverviewContent string

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Tusk Drift commands",
	Long:  utils.RenderMarkdown(driftOverviewContent),
}

func init() {
	rootCmd.AddCommand(driftCmd)
	driftCmd.PersistentFlags().StringVar(&cfgFile, "config", "", configFlagUsage)
	driftCmd.PersistentFlags().StringVar(&cfgOverrideFile, "config-override", "", configOverrideFlagUsage)
}

func bindLegacyDriftAliasConfigFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&cfgFile, "config", "", configFlagUsage)
}
