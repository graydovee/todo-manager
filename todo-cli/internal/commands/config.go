package commands

import (
	"fmt"

	"github.com/graydovee/todo-manager/todo-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect CLI configuration",
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "view",
		Short: "Print the effective CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			return writeResult(cmd, map[string]any{
				"api_key":  config.MaskAPIKey(appCtx.Config.APIKey),
				"base_url": appCtx.Config.BaseURL,
			})
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate CLI configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			appCtx := getAppContext(cmd)
			if appCtx == nil || appCtx.Config == nil {
				return writeError(cmd, fmt.Errorf("configuration is not loaded"))
			}
			err := config.Validate(appCtx.Config)
			if err != nil {
				return writeError(cmd, &ExitError{Code: 2, Err: err})
			}
			return writeResult(cmd, map[string]any{
				"valid":    true,
				"api_key":  config.MaskAPIKey(appCtx.Config.APIKey),
				"base_url": appCtx.Config.BaseURL,
			})
		},
	})
	return configCmd
}
