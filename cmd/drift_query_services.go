package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/Use-Tusk/tusk-cli/internal/api"
	"github.com/Use-Tusk/tusk-cli/internal/config"
	"github.com/spf13/cobra"
)

var driftQueryServicesCmd = &cobra.Command{
	Use:          "services",
	Short:        "List available Tusk Drift Cloud services",
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Ensure config is loaded with override support before Get() is called by SetupCloud
		if err := config.Load(cfgFile, cfgOverrideFile); err != nil {
			if cfgOverrideFile != "" || os.Getenv("TUSK_CONFIG_OVERRIDE") != "" {
				return fmt.Errorf("failed to load config: %w", err)
			}
		}

		client, authOptions, _, err := api.SetupCloud(context.Background(), false)
		if err != nil {
			return formatApiError(err)
		}

		result, err := client.ListDriftServices(context.Background(), authOptions)
		if err != nil {
			return formatApiError(err)
		}

		return printJSON(result)
	},
}

func init() {
	driftQueryCmd.AddCommand(driftQueryServicesCmd)
}
